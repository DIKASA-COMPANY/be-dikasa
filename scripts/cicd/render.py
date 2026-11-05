"""Deploy immutable GHCR images with Render's API and recover failed updates."""
import json
import os
import re
import subprocess
import time
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

import smoke


PENDING = {'created', 'queued', 'build_in_progress', 'pre_deploy_in_progress', 'update_in_progress'}
FAILED = {'build_failed', 'pre_deploy_failed', 'update_failed', 'canceled', 'deactivated'}


def pinned(image):
    return isinstance(image, str) and re.fullmatch(r'[^\s]+@sha256:[0-9a-f]{64}', image)


def repository(image):
    reference = image.split('@', 1)[0]
    return reference.rsplit(':', 1)[0] if ':' in reference.rsplit('/', 1)[-1] else reference


def resolved_image(deployment):
    """Use Render's resolved image SHA instead of trusting a mutable reference."""
    image = deployment.get('image') or {}
    reference, digest = image.get('ref', ''), image.get('sha', '')
    if not reference or not re.fullmatch(r'sha256:[0-9a-f]{64}', digest):
        raise ValueError('Render deployment must expose a resolved image SHA')
    return repository(reference) + '@' + digest


def latest(api, path):
    deployments = api('GET', path + '/deploys?limit=1')
    if not deployments:
        raise RuntimeError('Provision the initial Render deployment first')
    return deployments[0]['deploy']


def wait_live(api, path, deployment_id, image, sleep, attempts):
    """Verify the new deployment ID and resolved digest, not an older live deployment."""
    for _ in range(attempts):
        deployment = api('GET', path + '/deploys/' + deployment_id)
        if deployment.get('status') in FAILED:
            raise RuntimeError('Render deployment failed')
        if latest(api, path)['id'] != deployment_id:
            raise RuntimeError('Service changed outside this deployment')
        if deployment.get('status') == 'live':
            if resolved_image(deployment) != image:
                raise RuntimeError('Render resolved a different image digest')
            return
        sleep(5)
    raise RuntimeError('Timed out waiting for the requested Render deployment')


def deploy(api, service_id, image, migrate, smoke_test, rollback=False,
           sleep=time.sleep, attempts=120):
    """Deploy without changing service settings and restore the previous image on known failures."""
    if not pinned(image):
        raise ValueError('Deployment image must use a sha256 digest')
    path = f'/v1/services/{service_id}'
    service = api('GET', path)
    details = service.get('serviceDetails') or {}
    configured_image = service.get('imagePath')
    if (service.get('type') != 'web_service' or service.get('suspended') != 'not_suspended'
            or details.get('runtime') != 'image' or not configured_image
            or repository(configured_image) != repository(image)):
        raise ValueError('Use an unsuspended image-backed Render web service with the same GHCR repository')
    if (details.get('preDeployCommand')
            or (details.get('envSpecificDetails') or {}).get('dockerCommand')):
        raise ValueError('Clear Render pre-deploy and Docker command overrides; CI runs migrations')
    previous_latest = latest(api, path)
    if previous_latest.get('status') in PENDING:
        raise RuntimeError('A Render deployment is already pending')
    live = api('GET', path + '/deploys?status=live&limit=1')
    if not live:
        raise RuntimeError('Provision a live initial deployment for recovery first')
    previous_image = resolved_image(live[0]['deploy'])
    if repository(previous_image) != repository(image):
        raise ValueError('Previous live image does not match the release repository')
    if not rollback:
        migrate()
    if latest(api, path)['id'] != previous_latest['id']:
        raise RuntimeError('Service changed during migration; deployment aborted')
    updated_id = None
    try:
        updated_id = api('POST', path + '/deploys', {'imageUrl': image})['id']
        if not updated_id or updated_id == previous_latest['id']:
            raise RuntimeError('Render did not return a new deployment ID')
        wait_live(api, path, updated_id, image, sleep, attempts)
        smoke_test()
        if latest(api, path)['id'] != updated_id:
            raise RuntimeError('Service changed during smoke testing')
    except Exception:
        if updated_id and updated_id != previous_latest['id']:
            try:
                if latest(api, path)['id'] != updated_id:
                    raise RuntimeError('A newer deploy exists; refusing to overwrite it')
                state = api('GET', path + '/deploys/' + updated_id).get('status')
                if state in PENDING:
                    api('POST', path + '/deploys/' + updated_id + '/cancel')
                restored = api('POST', path + '/deploys', {'imageUrl': previous_image})['id']
                wait_live(api, path, restored, previous_image, sleep, attempts)
                print('Previous Render image restored')
            except Exception as recovery:
                raise RuntimeError('Deployment failed; recovery requires manual attention') from recovery
        # A lost create-deploy response has an unknown outcome. Never blindly retry it.
        raise


def api(method, path, body=None):
    """Keep API keys and provider response bodies out of job logs."""
    request = Request('https://api.render.com' + path, method=method,
                      data=json.dumps(body).encode() if body is not None else None,
                      headers={'Authorization': 'Bearer ' + os.environ['RENDER_API_KEY'],
                               'Content-Type': 'application/json', 'Accept': 'application/json'})
    try:
        with urlopen(request, timeout=30) as response:
            return json.load(response)
    except HTTPError as error:
        raise RuntimeError(f'Render API request failed (HTTP {error.code})') from None
    except URLError:
        raise RuntimeError('Render API connection failed; inspect deploy state before retrying') from None


def migrate_image(image):
    """Run SQL from the release digest and pass only the database secret into Docker."""
    environment = dict(os.environ, DATABASE_URL=os.environ['MIGRATION_DATABASE_URL'])
    subprocess.run(['docker', 'pull', image], check=True, timeout=300)
    subprocess.run(['docker', 'run', '--rm', '--env', 'DATABASE_URL', '--entrypoint', '/bin/sh',
                    image, '/app/deploy/migrate.sh'], env=environment, check=True, timeout=600)


def main():
    service_id = os.environ['RENDER_SERVICE_ID']
    if not re.fullmatch(r'srv-[a-z0-9]+', service_id):
        raise ValueError('RENDER_SERVICE_ID must be the srv- identifier from Render')
    image = os.environ['RELEASE_IMAGE']
    rollback = os.environ.get('ROLLBACK') == 'true'
    for key in ['RENDER_API_KEY', 'SMOKE_URL', 'SMOKE_USERNAME', 'SMOKE_PASSWORD']:
        if not os.environ.get(key):
            raise ValueError(f'Missing {key}')
    if not rollback and not os.environ.get('MIGRATION_DATABASE_URL'):
        raise ValueError('Missing MIGRATION_DATABASE_URL')
    smoke.validate_url(os.environ['SMOKE_URL'])
    deploy(api, service_id, image, lambda: migrate_image(image), smoke.main, rollback=rollback)
    with open(os.environ['GITHUB_OUTPUT'], 'a', encoding='utf-8') as destination:
        destination.write('service_url=' + os.environ['SMOKE_URL'].rstrip('/') + '\n')


if __name__ == '__main__':
    main()

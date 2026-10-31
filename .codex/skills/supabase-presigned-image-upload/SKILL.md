---
name: supabase-presigned-image-upload
description: Design, change, or review image-upload flows in the DIKASA backend so clients upload directly to Supabase Storage through server-issued signed upload URLs. Use whenever work adds or touches an image upload route, handler, service, storage adapter, API contract, or client-upload documentation; do not use for non-image files or signed download URLs.
---

# Supabase Presigned Image Upload

Apply these rules together with the repository's backend, authentication, TDD, and documentation guardrails.

## Required upload shape

- Use a two-phase flow: the authenticated business API first authorizes an upload intent and asks Supabase Storage for a signed upload URL; the client uploads the image bytes directly to Supabase; a completion call then verifies the stored object before its URL becomes business data.
- Do not accept or proxy image bytes through a DIKASA business endpoint. Do not use the Supabase secret key in a client. Only the backend may create signed upload URLs.
- Treat a Supabase signed **upload** URL as distinct from a signed download URL. Use the provider operation equivalent to `createSignedUploadUrl`, and use `uploadToSignedUrl` or its documented HTTP equivalent on the client.
- Keep PostgreSQL authoritative for the business reference. Persist only the final Supabase public URL after successful completion; never persist the temporary upload URL, signature/token, image bytes, or duplicate object metadata as the permanent image reference.

## Authorization and object naming

- Authenticate and authorize the actor, role, outlet, and target resource before issuing an upload intent and again before completion. Apply the repository's explicit public-route allowlist and tenant-isolation rules.
- Generate the bucket and object key on the backend. Never let a client choose an arbitrary bucket, outlet prefix, resource ID, or final object path.
- Use a collision-safe, tenant-scoped key such as `outlets/{outlet_id}/<feature>/<resource_id>/<purpose>/{uuid}.{ext}`. Prefer a new object key over overwrite/upsert so CDN caches and concurrent uploads cannot replace an existing image unexpectedly.
- Bind completion to the original authenticated actor/outlet/resource and server-generated object key. Do not accept a client-supplied URL as proof that an upload belongs to the request.
- Return only the minimum upload material the client needs, such as the signed upload URL or token plus its exact object path and the eventual public URL. Never return `SUPABASE_SECRET_KEY`, an authorization header, or any reusable server credential.
- Document the actual Supabase expiry. Do not invent a longer lifetime or claim the signed upload URL is revocable; handle expiration by requesting a new authorized intent.

## Validation and lifecycle

- At intent creation, reject unsupported declared MIME types and declared sizes outside the shared image policy, but treat all client declarations as untrusted hints.
- At completion, inspect the object from Supabase and apply the shared image validation to the real bytes or equivalent trustworthy server-observed data: file signature, allowed PNG/JPEG MIME, maximum byte size, dimensions, pixel count, and a non-empty object. Do not persist the URL until verification succeeds.
- Delete a newly uploaded object when verification fails or when the database mutation that adopts it fails. Restrict deletion to server-generated keys in the expected tenant/feature prefix.
- Make completion retry-safe. A repeated completion must not create duplicate business mutations, and concurrent intents must not allow an older upload to overwrite a newer adopted image.
- Define cleanup for abandoned, expired, and superseded objects. Prefer a bounded lifecycle job or bucket policy; do not silently leave permanent orphaned uploads.
- Preserve current replacement semantics: adopt the verified new object first, then delete the old object after the database update succeeds. Never remove the current image before the replacement is durable.

## API and tests

- Model upload intent and upload completion as explicit authenticated `/dikasa/v1` operations, or an equivalently explicit contract already established by the feature. A mutation that previously accepted multipart image bytes must be migrated so it references a completed upload instead.
- Keep domain services responsible for authorization and adoption. Keep Supabase signing, object inspection, public-URL construction, and deletion behind the shared storage boundary.
- Write failing tests before implementation. Cover intent authorization and tenant isolation, server-generated paths, invalid type/size hints, provider failures, response-secret leakage, expired or missing objects, tampered paths, real-content validation, idempotent completion, database-failure cleanup, replacement ordering, and orphan cleanup behavior.
- Fake Supabase in unit tests; never call live Storage. Assert that image bytes are absent from the business API and that the client upload target is Supabase, not the DIKASA API.
- Regenerate OpenAPI and update Bruno, README/development docs, and relevant architecture diagrams in the same implementation change. Document the client sequence, expiry behavior, allowed formats and limits, direct-to-Supabase upload, completion call, and stable error responses.

When existing documentation or code says uploads are backend-mediated, update it as part of the image-upload implementation; do not leave both architectures documented as current.

"""Test coverage handoff across Go module paths and separate CI jobs."""
from pathlib import Path
import tempfile
import unittest

from prepare_sonar import prepare


class CoverageTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        (self.root / 'go.mod').write_text('module be-dikasa\n\ngo 1.26.5\n', encoding='utf-8')
        self.source = self.root / 'internal/auth/service.go'
        self.source.parent.mkdir(parents=True)
        self.source.write_text('package auth\n', encoding='utf-8')
        self.report = self.root / 'coverage.out'
        self.output = self.root / 'coverage-sonar.out'

    def test_module_relative_and_absolute_paths_reference_the_same_source(self):
        for value in ['be-dikasa/internal/auth/service.go', 'internal/auth/service.go', self.source.as_posix()]:
            with self.subTest(path=value):
                self.report.write_text(f'mode: atomic\n{value}:1.1,1.2 1 7\n', encoding='utf-8')
                prepare(self.root, self.report, self.output)
                self.assertEqual(self.output.read_text(), 'mode: atomic\ninternal/auth/service.go:1.1,1.2 1 7\n')

    def test_windows_separators_are_normalized(self):
        self.report.write_text('mode: set\nbe-dikasa\\internal\\auth\\service.go:1.1,1.2 1 0\n')
        prepare(self.root, self.report, self.output)
        self.assertIn('internal/auth/service.go:1.1,1.2 1 0', self.output.read_text())

    def test_missing_report_is_rejected(self):
        with self.assertRaises(FileNotFoundError):
            prepare(self.root, self.report, self.output)

    def test_empty_or_invalid_coverage_cannot_be_reported_as_success(self):
        for value in ['', 'mode: atomic\n', 'mode: invalid\n', 'mode: atomic\nnot a coverage row\n']:
            with self.subTest(report=value):
                self.report.write_text(value)
                with self.assertRaises(ValueError):
                    prepare(self.root, self.report, self.output)

    def test_unknown_source_file_is_rejected(self):
        self.report.write_text('mode: atomic\nbe-dikasa/internal/missing.go:1.1,1.2 1 0\n')
        with self.assertRaises(ValueError):
            prepare(self.root, self.report, self.output)

    def test_path_outside_checkout_is_rejected(self):
        self.report.write_text('mode: atomic\n../outside.go:1.1,1.2 1 0\n')
        with self.assertRaises(ValueError):
            prepare(self.root, self.report, self.output)


if __name__ == '__main__':
    unittest.main()

from __future__ import annotations

import tempfile
import unittest
from pathlib import Path
from unittest import mock

from tools import python_proto_codegen


class PythonProtoCodegenTest(unittest.TestCase):
    def test_requirements_are_exactly_pinned(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            requirements = Path(temporary) / "requirements.txt"
            requirements.write_text(
                "grpcio==1.76.0\ngrpcio-tools==1.76.0\nprotobuf==6.31.1\n",
                encoding="utf-8",
            )
            self.assertEqual(
                python_proto_codegen.load_python_pins(requirements),
                python_proto_codegen.PINNED_VERSIONS,
            )
            requirements.write_text("grpcio>=1.76.0\n", encoding="utf-8")
            with self.assertRaisesRegex(
                python_proto_codegen.PythonCodegenError, "not pinned"
            ):
                python_proto_codegen.load_python_pins(requirements)

    @mock.patch("tools.python_proto_codegen.metadata.version")
    def test_installed_versions_must_match_pins(self, version: mock.Mock) -> None:
        version.side_effect = lambda name: python_proto_codegen.PINNED_VERSIONS[name]
        self.assertEqual(
            python_proto_codegen.validate_python_pins(),
            python_proto_codegen.PINNED_VERSIONS,
        )
        version.side_effect = lambda _name: "0.0.0"
        with self.assertRaisesRegex(
            python_proto_codegen.PythonCodegenError, "installed 0.0.0"
        ):
            python_proto_codegen.validate_python_pins()

    @mock.patch("tools.python_proto_codegen.grpc_include")
    @mock.patch("tools.python_proto_codegen.validate_python_pins")
    def test_generates_both_python_surfaces(
        self, validate: mock.Mock, include: mock.Mock
    ) -> None:
        include.return_value = Path("wkt")
        calls = []

        def run(command, cwd) -> None:
            calls.append((command, cwd))

        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary)
            proto = output / "base.proto"
            proto.write_text('syntax = "proto3";\n', encoding="utf-8")
            python_proto_codegen.generate_python_outputs(output, [proto], run)

        validate.assert_called_once_with()
        self.assertEqual(len(calls), 2)
        self.assertIn("--python_out=" + str(output / "python"), calls[0][0])
        self.assertIn("--python_out=" + str(output / "sdk-python"), calls[1][0])
        self.assertTrue(all(command[1:3] == ["-m", "grpc_tools.protoc"] for command, _ in calls))

    def test_repository_buf_template_does_not_override_python(self) -> None:
        template = (python_proto_codegen.REPO_ROOT / "buf.gen.yaml").read_text()
        self.assertNotIn("protocolbuffers/python", template)
        self.assertNotIn("grpc/python", template)
        self.assertEqual(
            python_proto_codegen.load_python_pins(),
            python_proto_codegen.PINNED_VERSIONS,
        )


if __name__ == "__main__":
    unittest.main()

import unittest

from cap.errors import (
    CAPError,
    MalformedPacketError,
    SignatureInvalidError,
    JobTimeoutError,
    InvalidInputError,
    SafetyDeniedError,
)


class TestTypedErrors(unittest.TestCase):
    def test_isinstance_cap_error(self):
        err = MalformedPacketError("bad wire bytes")
        self.assertIsInstance(err, CAPError)
        self.assertIsInstance(err, Exception)

    def test_error_attributes(self):
        err = MalformedPacketError("bad wire bytes")
        self.assertEqual(err.code, "ERROR_CODE_PROTOCOL_MALFORMED_PACKET")
        self.assertEqual(err.numeric_code, 101)
        self.assertEqual(str(err), "bad wire bytes")

    def test_subclass_codes(self):
        cases = [
            (SignatureInvalidError("bad"), "ERROR_CODE_PROTOCOL_SIGNATURE_INVALID", 103),
            (JobTimeoutError("slow"), "ERROR_CODE_JOB_TIMEOUT", 200),
            (InvalidInputError("missing"), "ERROR_CODE_JOB_INVALID_INPUT", 203),
            (SafetyDeniedError("blocked"), "ERROR_CODE_SAFETY_DENIED", 300),
        ]
        for err, code, numeric_code in cases:
            self.assertEqual(err.code, code)
            self.assertEqual(err.numeric_code, numeric_code)
            self.assertIsInstance(err, CAPError)

    def test_except_by_type(self):
        with self.assertRaises(MalformedPacketError):
            raise MalformedPacketError("test")

        with self.assertRaises(CAPError):
            raise MalformedPacketError("test")


if __name__ == "__main__":
    unittest.main()

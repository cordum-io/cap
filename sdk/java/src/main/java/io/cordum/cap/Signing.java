package io.cordum.cap;

import com.google.protobuf.ByteString;
import io.cordum.cap.agent.v1.BusPacket;

import java.security.GeneralSecurityException;
import java.security.KeyFactory;
import java.security.PrivateKey;
import java.security.PublicKey;
import java.security.Signature;
import java.security.SignatureException;
import java.security.spec.PKCS8EncodedKeySpec;
import java.security.spec.X509EncodedKeySpec;
import java.util.Base64;

public final class Signing {

    private Signing() {}

    /**
     * Sign a BusPacket using ECDSA P-256 with SHA-256.
     * Clears signature, serializes deterministically, signs, sets signature on builder.
     * ASN.1/DER format matches Go/Python/Node/C++ SDKs.
     */
    public static BusPacket signPacket(BusPacket packet, PrivateKey key)
            throws GeneralSecurityException {
        byte[] unsigned = Codec.marshalUnsignedForSignature(packet);

        Signature sig = Signature.getInstance("SHA256withECDSA");
        sig.initSign(key);
        sig.update(unsigned);
        byte[] signature = sig.sign();

        return packet.toBuilder()
                .setSignature(ByteString.copyFrom(signature))
                .build();
    }

    /**
     * Verify a BusPacket's ECDSA signature.
     * Returns true if signature is valid.
     */
    public static boolean verifyPacketSignature(BusPacket packet, PublicKey key)
            throws GeneralSecurityException {
        ByteString sigBytes = packet.getSignature();
        if (sigBytes.isEmpty()) return false;

        byte[] unsigned = Codec.marshalUnsignedForSignature(packet);

        Signature sig = Signature.getInstance("SHA256withECDSA");
        sig.initVerify(key);
        sig.update(unsigned);
        try {
            return sig.verify(sigBytes.toByteArray());
        } catch (SignatureException ignored) {
            return false;
        }
    }

    /**
     * Load a PEM-encoded PKCS8 EC private key.
     */
    public static PrivateKey loadPrivateKey(String pem) throws GeneralSecurityException {
        String base64 = stripPem(pem, "PRIVATE KEY");
        byte[] der = Base64.getDecoder().decode(base64);
        PKCS8EncodedKeySpec spec = new PKCS8EncodedKeySpec(der);
        return KeyFactory.getInstance("EC").generatePrivate(spec);
    }

    /**
     * Load a PEM-encoded X509 EC public key.
     */
    public static PublicKey loadPublicKey(String pem) throws GeneralSecurityException {
        String base64 = stripPem(pem, "PUBLIC KEY");
        byte[] der = Base64.getDecoder().decode(base64);
        X509EncodedKeySpec spec = new X509EncodedKeySpec(der);
        return KeyFactory.getInstance("EC").generatePublic(spec);
    }

    private static String stripPem(String pem, String label) {
        return pem.replaceAll("-----BEGIN " + label + "-----", "")
                  .replaceAll("-----END " + label + "-----", "")
                  .replaceAll("\\s+", "");
    }
}

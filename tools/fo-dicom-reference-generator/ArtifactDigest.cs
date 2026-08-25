using System.Security.Cryptography;

namespace FoDicomReferenceGenerator;

public sealed record ArtifactDigest(string Path, long Length, string Sha256)
{
    public static ArtifactDigest Create(string path, ReadOnlySpan<byte> content)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(path);

        var hash = SHA256.HashData(content);
        return new ArtifactDigest(path.Replace('\\', '/'), content.Length, Convert.ToHexStringLower(hash));
    }
}

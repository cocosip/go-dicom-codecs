namespace FoDicomReferenceGenerator;

public sealed class BundleArtifactStore
{
    private readonly string _root;
    private readonly HashSet<string> _paths = new(StringComparer.Ordinal);
    private readonly List<ArtifactDigest> _artifacts = new();

    public BundleArtifactStore(string root)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(root);
        _root = Path.GetFullPath(root);
    }

    public IReadOnlyList<ArtifactDigest> Artifacts => _artifacts;

    public ArtifactDigest Write(string relativePath, ReadOnlySpan<byte> content)
    {
        var normalized = NormalizeRelativePath(relativePath);
        if (!_paths.Add(normalized))
        {
            throw new InvalidOperationException($"Bundle artifact '{normalized}' was written more than once.");
        }

        var fullPath = Path.GetFullPath(Path.Combine(_root, normalized.Replace('/', Path.DirectorySeparatorChar)));
        var rootPrefix = _root.TrimEnd(Path.DirectorySeparatorChar) + Path.DirectorySeparatorChar;
        if (!fullPath.StartsWith(rootPrefix, StringComparison.OrdinalIgnoreCase))
        {
            throw new ArgumentException($"Bundle artifact path '{relativePath}' escapes the output directory.");
        }

        Directory.CreateDirectory(Path.GetDirectoryName(fullPath)!);
        File.WriteAllBytes(fullPath, content);

        var digest = ArtifactDigest.Create(normalized, content);
        _artifacts.Add(digest);
        return digest;
    }

    private static string NormalizeRelativePath(string path)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(path);
        if (Path.IsPathRooted(path))
        {
            throw new ArgumentException("Bundle artifact paths must be relative.");
        }

        return path.Replace('\\', '/');
    }
}

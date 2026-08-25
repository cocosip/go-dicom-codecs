using System.Reflection;
using System.Security.Cryptography;
using System.Text;

namespace FoDicomReferenceGenerator;

public sealed record GeneratorSourceFile(string Path, byte[] Content);

public static class GeneratorSourceHasher
{
    private const string ResourcePrefix = "generator-source/";

    public static string Compute(IEnumerable<GeneratorSourceFile> files)
    {
        ArgumentNullException.ThrowIfNull(files);

        using var hash = IncrementalHash.CreateHash(HashAlgorithmName.SHA256);
        foreach (var file in files.OrderBy(file => file.Path, StringComparer.Ordinal))
        {
            var path = Encoding.UTF8.GetBytes(file.Path.Replace('\\', '/'));
            hash.AppendData(path);
            hash.AppendData(new byte[] { 0 });
            hash.AppendData(file.Content);
            hash.AppendData(new byte[] { 0 });
        }
        return Convert.ToHexStringLower(hash.GetHashAndReset());
    }

    public static string FromAssembly(Assembly assembly)
    {
        ArgumentNullException.ThrowIfNull(assembly);

        var files = assembly.GetManifestResourceNames()
            .Where(name => name.StartsWith(ResourcePrefix, StringComparison.Ordinal))
            .Select(name => new GeneratorSourceFile(
                name[ResourcePrefix.Length..],
                ReadResource(assembly, name)))
            .ToArray();
        if (files.Length == 0)
        {
            throw new InvalidOperationException("Generator source resources are missing from the assembly.");
        }
        return Compute(files);
    }

    private static byte[] ReadResource(Assembly assembly, string name)
    {
        using var stream = assembly.GetManifestResourceStream(name)
            ?? throw new InvalidOperationException($"Generator source resource '{name}' cannot be read.");
        using var memory = new MemoryStream();
        stream.CopyTo(memory);
        return memory.ToArray();
    }
}

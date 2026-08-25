namespace FoDicomReferenceGenerator;

public sealed record GeneratorOptions(
    IReadOnlyList<string> InputPaths,
    string OutputDirectory,
    string SourceCommit)
{
    public GeneratorOptions(string inputPath, string outputDirectory, string sourceCommit)
        : this(new[] { inputPath }, outputDirectory, sourceCommit)
    {
    }

    public static GeneratorOptions Parse(string[] args)
    {
        var inputs = new List<string>();
        string? output = null;
        string? sourceCommit = null;

        for (var index = 0; index < args.Length; index++)
        {
            var option = args[index];
            switch (option)
            {
                case "--input":
                    inputs.Add(NextValue(args, ref index, option));
                    break;
                case "--output":
                    output = NextValue(args, ref index, option);
                    break;
                case "--source-commit":
                    sourceCommit = NextValue(args, ref index, option);
                    break;
                default:
                    if (option.Contains("native", StringComparison.OrdinalIgnoreCase))
                    {
                        throw new ArgumentException("Direct native library arguments are not supported.");
                    }
                    throw new ArgumentException($"Unknown option '{option}'.");
            }
        }

        if (inputs.Count == 0
            || string.IsNullOrWhiteSpace(output)
            || string.IsNullOrWhiteSpace(sourceCommit))
        {
            throw new ArgumentException(
                "Usage: --input <source.dcm> [--input <source.dcm> ...] " +
                "--output <bundle-dir> --source-commit <40-hex>.");
        }
        if (sourceCommit.Length != 40 || sourceCommit.Any(c => !Uri.IsHexDigit(c)))
        {
            throw new ArgumentException("--source-commit must be a 40-character hexadecimal commit.");
        }

        return new GeneratorOptions(
            inputs.Select(Path.GetFullPath).ToArray(),
            Path.GetFullPath(output),
            sourceCommit.ToLowerInvariant());
    }

    private static string NextValue(string[] args, ref int index, string option)
    {
        if (++index >= args.Length || args[index].StartsWith("--", StringComparison.Ordinal))
        {
            throw new ArgumentException($"{option} requires a value.");
        }
        return args[index];
    }
}

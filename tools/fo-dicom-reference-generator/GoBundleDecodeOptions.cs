namespace FoDicomReferenceGenerator;

public sealed record GoBundleDecodeOptions(
    string BundleDirectory,
    string SourceCommit)
{
    public static GoBundleDecodeOptions Parse(string[] args)
    {
        string? bundle = null;
        string? sourceCommit = null;

        for (var index = 0; index < args.Length; index++)
        {
            var option = args[index];
            switch (option)
            {
                case "--decode-go-bundle":
                    bundle = NextValue(args, ref index, option);
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

        if (string.IsNullOrWhiteSpace(bundle) || string.IsNullOrWhiteSpace(sourceCommit))
        {
            throw new ArgumentException(
                "Usage: --decode-go-bundle <bundle-dir> --source-commit <40-hex>.");
        }
        if (sourceCommit.Length != 40 || sourceCommit.Any(c => !Uri.IsHexDigit(c)))
        {
            throw new ArgumentException("--source-commit must be a 40-character hexadecimal commit.");
        }

        return new GoBundleDecodeOptions(
            Path.GetFullPath(bundle),
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

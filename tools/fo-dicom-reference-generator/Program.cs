namespace FoDicomReferenceGenerator;

public static class Program
{
    public static int Main(string[] args)
    {
        try
        {
            if (args.Length > 0
                && string.Equals(args[0], "--write-matrix-sources", StringComparison.Ordinal))
            {
                if (args.Length != 2)
                {
                    throw new ArgumentException(
                        "Usage: --write-matrix-sources <empty-output-directory>.");
                }
                var paths = FixtureMatrixSourceWriter.Write(Path.GetFullPath(args[1]));
                Console.WriteLine($"Generated {paths.Count} deterministic DICOM source fixtures.");
                return 0;
            }

            if (args.Length > 0
                && string.Equals(args[0], "--decode-go-bundle", StringComparison.Ordinal))
            {
                var decodeOptions = GoBundleDecodeOptions.Parse(args);
                var decodedManifest = GoBundleDecoder.Decode(decodeOptions);
                Console.WriteLine(
                    $"Decoded {decodedManifest.Fixtures.Count * 3} saved Go HTJ2K files with " +
                    $"fo-dicom.Codecs {decodedManifest.Codec.ManagedVersion}.");
                return 0;
            }

            var options = GeneratorOptions.Parse(args);
            var manifest = ReferenceBundleGenerator.Generate(options);
            Console.WriteLine(
                $"Generated {manifest.Fixtures.Count * 3} HTJ2K references for " +
                $"{manifest.Fixtures.Count} fixtures with " +
                $"fo-dicom.Codecs {manifest.Codec.ManagedVersion} at '{options.OutputDirectory}'.");
            return 0;
        }
        catch (Exception exception)
        {
            Console.Error.WriteLine(exception.Message);
            return 1;
        }
    }
}

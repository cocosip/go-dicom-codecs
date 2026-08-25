using System.Text;
using FellowOakDicom;
using FellowOakDicom.Imaging;
using FellowOakDicom.Imaging.Codec;
using FellowOakDicom.Imaging.NativeCodec;

namespace FoDicomReferenceGenerator;

public static class ReferenceBundleGenerator
{
    private sealed record SyntaxDefinition(
        string Name,
        DicomTransferSyntax TransferSyntax,
        bool Lossless);

    private static readonly SyntaxDefinition[] Syntaxes =
    {
        new("htj2k-lossless", DicomTransferSyntax.HTJ2KLossless, true),
        new("htj2k-lossless-rpcl", DicomTransferSyntax.HTJ2KLosslessRPCL, true),
        new("htj2k-lossy", DicomTransferSyntax.HTJ2K, false)
    };

    public static ReferenceBundleManifest Generate(GeneratorOptions options)
    {
        ArgumentNullException.ThrowIfNull(options);
        foreach (var inputPath in options.InputPaths)
        {
            if (!File.Exists(inputPath))
            {
                throw new FileNotFoundException("Source DICOM file was not found.", inputPath);
            }
        }
        if (Directory.Exists(options.OutputDirectory)
            && Directory.EnumerateFileSystemEntries(options.OutputDirectory).Any())
        {
            throw new InvalidOperationException(
                $"Output directory '{options.OutputDirectory}' must be empty.");
        }

        var codec = CodecProvenance.FromAssembly(
            typeof(NativeTranscoderManager).Assembly,
            options.SourceCommit);
        new DicomSetupBuilder()
            .RegisterServices(services => services
                .AddFellowOakDicom()
                .AddTranscoderManager<NativeTranscoderManager>())
            .SkipValidation()
            .Build();

        var names = options.InputPaths
            .Select(inputPath => SanitizeName(Path.GetFileNameWithoutExtension(inputPath)))
            .ToArray();
        var duplicateName = names
            .GroupBy(name => name, StringComparer.Ordinal)
            .FirstOrDefault(group => group.Count() > 1)?.Key;
        if (duplicateName is not null)
        {
            throw new InvalidOperationException(
                $"Source DICOM files resolve to duplicate fixture name '{duplicateName}'.");
        }

        var store = new BundleArtifactStore(options.OutputDirectory);
        var fixtures = options.InputPaths
            .Zip(names, (inputPath, name) => GenerateFixture(store, inputPath, name))
            .ToArray();
        var manifest = new ReferenceBundleManifest(
            2,
            codec,
            fixtures,
            store.Artifacts.OrderBy(artifact => artifact.Path, StringComparer.Ordinal).ToArray());

        Directory.CreateDirectory(options.OutputDirectory);
        File.WriteAllText(
            Path.Combine(options.OutputDirectory, "manifest.json"),
            ReferenceManifestJson.Serialize(manifest),
            new UTF8Encoding(encoderShouldEmitUTF8Identifier: false));
        return manifest;
    }

    private static ReferenceFixtureArtifacts GenerateFixture(
        BundleArtifactStore store,
        string inputPath,
        string name)
    {
        var sourceFile = DicomFile.Open(inputPath, FileReadOption.ReadAll);
        if (sourceFile.Dataset.InternalTransferSyntax.IsEncapsulated)
        {
            throw new InvalidOperationException(
                "Reference generation requires an uncompressed source DICOM file.");
        }

        var sourcePixelData = DicomPixelData.Create(sourceFile.Dataset);
        var sourceDicomPath = $"sources/{name}.dcm";
        store.Write(sourceDicomPath, File.ReadAllBytes(inputPath));
        var sourceFrames = WriteRawFrames(
            store,
            sourcePixelData,
            $"sources/frames/{name}");
        var encoderInputFrames = WriteEncoderInputFrames(store, sourcePixelData, name, sourceFrames);

        var syntaxArtifacts = new List<ReferenceSyntaxArtifacts>(Syntaxes.Length);
        foreach (var syntax in Syntaxes)
        {
            syntaxArtifacts.Add(GenerateSyntax(store, sourceFile, name, syntax));
        }

        var image = new ReferenceImageMetadata(
            name,
            sourcePixelData.Width,
            sourcePixelData.Height,
            sourcePixelData.SamplesPerPixel,
            sourcePixelData.BitsAllocated,
            sourcePixelData.BitsStored,
            sourcePixelData.PixelRepresentation == PixelRepresentation.Signed,
            sourcePixelData.PhotometricInterpretation?.Value ?? "unknown",
            sourcePixelData.NumberOfFrames);
        return new ReferenceFixtureArtifacts(
            image,
            new ReferenceSourceArtifacts(sourceDicomPath, sourceFrames, encoderInputFrames),
            syntaxArtifacts);
    }

    private static ReferenceSyntaxArtifacts GenerateSyntax(
        BundleArtifactStore store,
        DicomFile sourceFile,
        string name,
        SyntaxDefinition syntax)
    {
        var encoded = new DicomTranscoder(
            sourceFile.Dataset.InternalTransferSyntax,
            syntax.TransferSyntax)
            .Transcode(sourceFile);

        var encodedDicomPath = $"fo-encoded/{name}-{syntax.Name}.dcm";
        store.Write(encodedDicomPath, SaveToBytes(encoded));

        var encodedPixelData = DicomPixelData.Create(encoded.Dataset);
        var encodedFrames = new List<string>(encodedPixelData.NumberOfFrames);
        for (var frame = 0; frame < encodedPixelData.NumberOfFrames; frame++)
        {
            var path = $"fo-encoded/frames/{name}-{syntax.Name}-frame-{frame:D4}.j2c";
            store.Write(path, CodestreamExtractor.Extract(encodedPixelData.GetFrame(frame).Data));
            encodedFrames.Add(path);
        }

        if (encodedPixelData.PhotometricInterpretation == PhotometricInterpretation.YbrFull
            || encodedPixelData.PhotometricInterpretation == PhotometricInterpretation.YbrFull422)
        {
            encodedPixelData.PhotometricInterpretation = PhotometricInterpretation.Rgb;
        }

        var decoded = ReferenceFrameDecoder.Decode(encoded);
        var decodedDicomPath = $"decoded/fo-from-fo/{name}-{syntax.Name}.dcm";
        store.Write(decodedDicomPath, SaveToBytes(decoded));
        var decodedFrames = WriteRawFrames(
            store,
            DicomPixelData.Create(decoded.Dataset),
            $"decoded/fo-from-fo/{name}-{syntax.Name}");

        return new ReferenceSyntaxArtifacts(
            syntax.Name,
            syntax.TransferSyntax.UID.UID,
            syntax.Lossless,
            encodedDicomPath,
            encodedFrames,
            decodedDicomPath,
            decodedFrames);
    }

    private static IReadOnlyList<string> WriteRawFrames(
        BundleArtifactStore store,
        DicomPixelData pixelData,
        string pathPrefix)
    {
        var paths = new List<string>(pixelData.NumberOfFrames);
        for (var frame = 0; frame < pixelData.NumberOfFrames; frame++)
        {
            var bytes = pixelData.GetFrame(frame).Data;
            if (bytes.Length < pixelData.UncompressedFrameSize)
            {
                throw new InvalidDataException(
                    $"Frame {frame} contains {bytes.Length} bytes; expected {pixelData.UncompressedFrameSize}.");
            }

            var path = $"{pathPrefix}-frame-{frame:D4}.raw";
            store.Write(path, bytes.AsSpan(0, pixelData.UncompressedFrameSize));
            paths.Add(path);
        }
        return paths;
    }

    private static IReadOnlyList<string> WriteEncoderInputFrames(
        BundleArtifactStore store,
        DicomPixelData pixelData,
        string name,
        IReadOnlyList<string> sourceFrames)
    {
        if (pixelData.PhotometricInterpretation != PhotometricInterpretation.YbrFull
            && pixelData.PhotometricInterpretation != PhotometricInterpretation.YbrFull422)
        {
            return sourceFrames;
        }

        var paths = new List<string>(pixelData.NumberOfFrames);
        for (var frame = 0; frame < pixelData.NumberOfFrames; frame++)
        {
            var source = pixelData.GetFrame(frame);
            var converted = pixelData.PhotometricInterpretation == PhotometricInterpretation.YbrFull
                ? PixelDataConverter.YbrFullToRgb(source)
                : PixelDataConverter.YbrFull422ToRgb(source, pixelData.Width);
            var path = $"sources/encoder-input-frames/{name}-frame-{frame:D4}.rgb";
            store.Write(path, converted.Data);
            paths.Add(path);
        }
        return paths;
    }

    private static byte[] SaveToBytes(DicomFile file)
    {
        using var stream = new MemoryStream();
        file.Save(stream);
        return stream.ToArray();
    }

    private static string SanitizeName(string value)
    {
        var characters = value.Select(character =>
            char.IsAsciiLetterOrDigit(character) || character is '-' or '_'
                ? character
                : '-').ToArray();
        var sanitized = new string(characters).Trim('-');
        return sanitized.Length > 0 ? sanitized : "source";
    }
}

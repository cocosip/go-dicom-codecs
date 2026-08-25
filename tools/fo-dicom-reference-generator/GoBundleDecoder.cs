using System.Text;
using FellowOakDicom;
using FellowOakDicom.Imaging;
using FellowOakDicom.Imaging.Codec;
using FellowOakDicom.Imaging.NativeCodec;

namespace FoDicomReferenceGenerator;

public static class GoBundleDecoder
{
    public static ReferenceBundleManifest Decode(GoBundleDecodeOptions options)
    {
        ArgumentNullException.ThrowIfNull(options);
        var manifestPath = Path.Combine(options.BundleDirectory, "manifest.json");
        var manifest = ReferenceManifestJson.Deserialize(File.ReadAllText(manifestPath));
        if (manifest.SchemaVersion != 2)
        {
            throw new InvalidDataException($"Schema version {manifest.SchemaVersion} is not supported.");
        }

        var loadedCodec = CodecProvenance.FromAssembly(
            typeof(NativeTranscoderManager).Assembly,
            options.SourceCommit);
        if (loadedCodec != manifest.Codec)
        {
            throw new InvalidOperationException(
                $"Loaded fo-dicom.Codecs provenance {loadedCodec.ManagedVersion}/{loadedCodec.SourceCommit} " +
                $"does not match manifest {manifest.Codec.ManagedVersion}/{manifest.Codec.SourceCommit}.");
        }

        new DicomSetupBuilder()
            .RegisterServices(services => services
                .AddFellowOakDicom()
                .AddTranscoderManager<NativeTranscoderManager>())
            .SkipValidation()
            .Build();

        var store = new BundleArtifactStore(options.BundleDirectory);
        var fixtures = manifest.Fixtures
            .Select(fixture => DecodeFixture(options.BundleDirectory, store, fixture))
            .ToArray();
        var replacementPaths = store.Artifacts
            .Select(artifact => artifact.Path)
            .ToHashSet(StringComparer.Ordinal);
        var artifacts = manifest.Artifacts
            .Where(artifact => !replacementPaths.Contains(artifact.Path))
            .Concat(store.Artifacts)
            .OrderBy(artifact => artifact.Path, StringComparer.Ordinal)
            .ToArray();
        var updated = manifest with
        {
            Fixtures = fixtures,
            Artifacts = artifacts
        };
        File.WriteAllText(
            manifestPath,
            ReferenceManifestJson.Serialize(updated),
            new UTF8Encoding(encoderShouldEmitUTF8Identifier: false));
        return updated;
    }

    private static ReferenceFixtureArtifacts DecodeFixture(
        string bundleRoot,
        BundleArtifactStore store,
        ReferenceFixtureArtifacts fixture)
    {
        var syntaxes = fixture.Syntaxes
            .Select(syntax => DecodeSyntax(bundleRoot, store, fixture.Image, syntax))
            .ToArray();
        return fixture with { Syntaxes = syntaxes };
    }

    private static ReferenceSyntaxArtifacts DecodeSyntax(
        string bundleRoot,
        BundleArtifactStore store,
        ReferenceImageMetadata image,
        ReferenceSyntaxArtifacts syntax)
    {
        if (string.IsNullOrWhiteSpace(syntax.GoEncodedDicom))
        {
            throw new InvalidDataException(
                $"Fixture '{image.Name}' syntax '{syntax.Name}' has no Go-encoded DICOM path.");
        }
        var encodedPath = ResolveBundlePath(bundleRoot, syntax.GoEncodedDicom, "go-encoded/", ".dcm");
        var encoded = DicomFile.Open(encodedPath, FileReadOption.ReadAll);
        if (encoded.Dataset.InternalTransferSyntax.UID.UID != syntax.TransferSyntaxUid)
        {
            throw new InvalidDataException(
                $"Go-encoded DICOM '{syntax.GoEncodedDicom}' transfer syntax is " +
                $"'{encoded.Dataset.InternalTransferSyntax.UID.UID}', expected '{syntax.TransferSyntaxUid}'.");
        }

        var encodedPixelData = DicomPixelData.Create(encoded.Dataset);
        if (encodedPixelData.NumberOfFrames != image.FrameCount)
        {
            throw new InvalidDataException(
                $"Go-encoded DICOM '{syntax.GoEncodedDicom}' has {encodedPixelData.NumberOfFrames} frames; " +
                $"expected {image.FrameCount}.");
        }
        if (encodedPixelData.PhotometricInterpretation == PhotometricInterpretation.YbrFull
            || encodedPixelData.PhotometricInterpretation == PhotometricInterpretation.YbrFull422)
        {
            encodedPixelData.PhotometricInterpretation = PhotometricInterpretation.Rgb;
        }

        var decoded = ReferenceFrameDecoder.Decode(encoded);
        var prefix = $"decoded/fo-from-go/{image.Name}-{syntax.Name}";
        var decodedDicomPath = prefix + ".dcm";
        store.Write(decodedDicomPath, SaveToBytes(decoded));
        var decodedFrames = WriteRawFrames(store, DicomPixelData.Create(decoded.Dataset), prefix);
        if (decodedFrames.Count != image.FrameCount)
        {
            throw new InvalidDataException(
                $"Decoded DICOM '{decodedDicomPath}' has {decodedFrames.Count} frames; expected {image.FrameCount}.");
        }
        return syntax with
        {
            FoFromGoDicom = decodedDicomPath,
            FoFromGoFrames = decodedFrames
        };
    }

    private static IReadOnlyList<string> WriteRawFrames(
        BundleArtifactStore store,
        DicomPixelData pixelData,
        string prefix)
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
            var path = $"{prefix}-frame-{frame:D4}.raw";
            store.Write(path, bytes.AsSpan(0, pixelData.UncompressedFrameSize));
            paths.Add(path);
        }
        return paths;
    }

    private static string ResolveBundlePath(
        string bundleRoot,
        string relativePath,
        string requiredPrefix,
        string requiredExtension)
    {
        if (Path.IsPathRooted(relativePath)
            || relativePath.Contains('\\')
            || !relativePath.StartsWith(requiredPrefix, StringComparison.Ordinal)
            || !string.Equals(Path.GetExtension(relativePath), requiredExtension, StringComparison.Ordinal))
        {
            throw new InvalidDataException($"Unsafe bundle path '{relativePath}'.");
        }
        var root = Path.GetFullPath(bundleRoot).TrimEnd(Path.DirectorySeparatorChar);
        var fullPath = Path.GetFullPath(Path.Combine(root, relativePath.Replace('/', Path.DirectorySeparatorChar)));
        var rootPrefix = root + Path.DirectorySeparatorChar;
        if (!fullPath.StartsWith(rootPrefix, StringComparison.OrdinalIgnoreCase))
        {
            throw new InvalidDataException($"Bundle path '{relativePath}' escapes the bundle directory.");
        }
        return fullPath;
    }

    private static byte[] SaveToBytes(DicomFile file)
    {
        using var stream = new MemoryStream();
        file.Save(stream);
        return stream.ToArray();
    }
}

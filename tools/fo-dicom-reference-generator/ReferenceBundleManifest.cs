using System.Text.Json;

namespace FoDicomReferenceGenerator;

public sealed record ReferenceBundleManifest(
    int SchemaVersion,
    CodecProvenance Codec,
    IReadOnlyList<ReferenceFixtureArtifacts> Fixtures,
    IReadOnlyList<ArtifactDigest> Artifacts);

public sealed record ReferenceFixtureArtifacts(
    ReferenceImageMetadata Image,
    ReferenceSourceArtifacts Source,
    IReadOnlyList<ReferenceSyntaxArtifacts> Syntaxes);

public sealed record ReferenceImageMetadata(
    string Name,
    int Width,
    int Height,
    int SamplesPerPixel,
    int BitsAllocated,
    int BitsStored,
    bool Signed,
    string PhotometricInterpretation,
    int FrameCount);

public sealed record ReferenceSourceArtifacts(
    string Dicom,
    IReadOnlyList<string> Frames,
    IReadOnlyList<string> EncoderInputFrames);

public sealed record ReferenceSyntaxArtifacts(
    string Name,
    string TransferSyntaxUid,
    bool Lossless,
    string EncodedDicom,
    IReadOnlyList<string> EncodedFrames,
    string DecodedDicom,
    IReadOnlyList<string> DecodedFrames,
    string? GoEncodedDicom = null,
    IReadOnlyList<string>? GoEncodedFrames = null,
    string? GoFromGoDicom = null,
    IReadOnlyList<string>? GoFromGoFrames = null,
    string? GoFromFoDicom = null,
    IReadOnlyList<string>? GoFromFoFrames = null,
    string? FoFromGoDicom = null,
    IReadOnlyList<string>? FoFromGoFrames = null);

public static class ReferenceManifestJson
{
    private static readonly JsonSerializerOptions Options = new(JsonSerializerDefaults.Web)
    {
        WriteIndented = true
    };

    public static string Serialize(ReferenceBundleManifest manifest)
    {
        ArgumentNullException.ThrowIfNull(manifest);
        return JsonSerializer.Serialize(manifest, Options) + Environment.NewLine;
    }

    public static ReferenceBundleManifest Deserialize(string json)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(json);
        return JsonSerializer.Deserialize<ReferenceBundleManifest>(json, Options)
            ?? throw new InvalidDataException("Manifest JSON is empty.");
    }
}

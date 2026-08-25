using Microsoft.VisualStudio.TestTools.UnitTesting;
using FellowOakDicom;
using FellowOakDicom.Imaging;
using FellowOakDicom.Imaging.NativeCodec;

namespace FoDicomReferenceGenerator.Tests;

[TestClass]
public sealed class ReferenceGeneratorContractTests
{
    [TestMethod]
    [DataRow("6.0.0-beta1")]
    [DataRow("6.0.0")]
    [DataRow("6.9.9")]
    public void AcceptedVersionRangeIncludesSixSeries(string value)
    {
        Assert.IsTrue(ReferenceVersionPolicy.IsAccepted(value));
    }

    [TestMethod]
    [DataRow("5.16.7")]
    [DataRow("7.0.0-alpha1")]
    [DataRow("7.0.0")]
    public void AcceptedVersionRangeRejectsOutsideVersions(string value)
    {
        Assert.IsFalse(ReferenceVersionPolicy.IsAccepted(value));
    }

    [TestMethod]
    public void OptionsRejectUnknownDllArguments()
    {
        var exception = Assert.ThrowsExactly<ArgumentException>(() =>
            GeneratorOptions.Parse(new[]
            {
                "--input", "source.dcm",
                "--output", "bundle",
                "--source-commit", new string('a', 40),
                "--codec-dll", "codec.dll"
            }));

        StringAssert.Contains(exception.Message, "Unknown option");
    }

    [TestMethod]
    public void OptionsRequireInputOutputAndSourceCommit()
    {
        var options = GeneratorOptions.Parse(new[]
        {
            "--input", "source.dcm",
            "--output", "bundle",
            "--source-commit", new string('a', 40)
        });

        CollectionAssert.AreEqual(
            new[] { Path.GetFullPath("source.dcm") },
            options.InputPaths.ToArray());
        Assert.AreEqual(Path.GetFullPath("bundle"), options.OutputDirectory);
        Assert.AreEqual(new string('a', 40), options.SourceCommit);
    }

    [TestMethod]
    public void OptionsAcceptRepeatedInputsInCommandLineOrder()
    {
        var options = GeneratorOptions.Parse(new[]
        {
            "--input", "source-b.dcm",
            "--input", "source-a.dcm",
            "--output", "bundle",
            "--source-commit", new string('a', 40)
        });

        CollectionAssert.AreEqual(
            new[]
            {
                Path.GetFullPath("source-b.dcm"),
                Path.GetFullPath("source-a.dcm")
            },
            options.InputPaths.ToArray());
    }

    [TestMethod]
    public void GoBundleDecodeOptionsRequireBundleAndRejectUnknownDllArguments()
    {
        var options = GoBundleDecodeOptions.Parse(new[]
        {
            "--decode-go-bundle", "bundle",
            "--source-commit", new string('a', 40)
        });

        Assert.AreEqual(Path.GetFullPath("bundle"), options.BundleDirectory);
        Assert.AreEqual(new string('a', 40), options.SourceCommit);

        var exception = Assert.ThrowsExactly<ArgumentException>(() =>
            GoBundleDecodeOptions.Parse(new[]
            {
                "--decode-go-bundle", "bundle",
                "--source-commit", new string('a', 40),
                "--codec-dll", "codec.dll"
            }));
        StringAssert.Contains(exception.Message, "Unknown option");
    }

    [TestMethod]
    public void ArtifactDigestRecordsExactLengthAndLowercaseSha256()
    {
        var digest = ArtifactDigest.Create("fo-encoded/frames/frame-0000.j2c", new byte[] { 0, 1, 2, 3 });

        Assert.AreEqual("fo-encoded/frames/frame-0000.j2c", digest.Path);
        Assert.AreEqual(4, digest.Length);
        Assert.AreEqual(
            "054edec1d0211f624fed0cbca9d4f9400b0e491c43742af2c5b0abebf0c990d8",
            digest.Sha256);
    }

    [TestMethod]
    public void LoadedCodecProvenanceUsesActualManagedAssemblyVersion()
    {
        var provenance = CodecProvenance.FromAssembly(
            typeof(NativeTranscoderManager).Assembly,
            new string('b', 40));

        Assert.IsTrue(ReferenceVersionPolicy.IsAccepted(provenance.ManagedVersion));
        Assert.AreEqual(new string('b', 40), provenance.SourceCommit);
        StringAssert.Contains(provenance.AssemblyName, "fo-dicom.Codecs");
    }

    [TestMethod]
    public void CodecProvenanceRejectsManagedVersionOutsideAcceptedRange()
    {
        Assert.ThrowsExactly<InvalidOperationException>(() =>
            CodecProvenance.Create("fo-dicom.Codecs", "5.16.7", new string('c', 40)));
    }

    [TestMethod]
    public void CodestreamExtractionStopsAtEocAndRemovesDicomPadding()
    {
        var extracted = CodestreamExtractor.Extract(new byte[]
        {
            0xff, 0x4f, 0x01, 0x02, 0xff, 0xd9, 0x00
        });

        CollectionAssert.AreEqual(
            new byte[] { 0xff, 0x4f, 0x01, 0x02, 0xff, 0xd9 },
            extracted);
    }

    [TestMethod]
    public void CodestreamExtractionRejectsMissingEoc()
    {
        Assert.ThrowsExactly<InvalidDataException>(() =>
            CodestreamExtractor.Extract(new byte[] { 0xff, 0x4f, 0x01, 0x02 }));
    }

    [TestMethod]
    public void BundleArtifactStoreWritesTheBytesItHashes()
    {
        var root = Path.Combine(Path.GetTempPath(), Guid.NewGuid().ToString("N"));
        try
        {
            var store = new BundleArtifactStore(root);
            var digest = store.Write("fo-encoded/frames/frame-0000.j2c", new byte[] { 0, 1, 2, 3 });

            CollectionAssert.AreEqual(
                new byte[] { 0, 1, 2, 3 },
                File.ReadAllBytes(Path.Combine(root, "fo-encoded", "frames", "frame-0000.j2c")));
            Assert.AreEqual("054edec1d0211f624fed0cbca9d4f9400b0e491c43742af2c5b0abebf0c990d8", digest.Sha256);
            CollectionAssert.AreEqual(new[] { digest }, store.Artifacts.ToArray());
        }
        finally
        {
            if (Directory.Exists(root))
            {
                Directory.Delete(root, recursive: true);
            }
        }
    }

    [TestMethod]
    public void GeneratorSourceHashIsOrderIndependentAndContentSensitive()
    {
        var first = GeneratorSourceHasher.Compute(new[]
        {
            new GeneratorSourceFile("b.cs", new byte[] { 2 }),
            new GeneratorSourceFile("a.cs", new byte[] { 1 })
        });
        var reordered = GeneratorSourceHasher.Compute(new[]
        {
            new GeneratorSourceFile("a.cs", new byte[] { 1 }),
            new GeneratorSourceFile("b.cs", new byte[] { 2 })
        });
        var changed = GeneratorSourceHasher.Compute(new[]
        {
            new GeneratorSourceFile("a.cs", new byte[] { 1 }),
            new GeneratorSourceFile("b.cs", new byte[] { 3 })
        });

        Assert.AreEqual(first, reordered);
        Assert.AreNotEqual(first, changed);
        Assert.AreEqual(64, first.Length);
    }

    [TestMethod]
    public void ReferenceManifestJsonRecordsProvenanceSyntaxFramesAndHashes()
    {
        var manifest = new ReferenceBundleManifest(
            SchemaVersion: 2,
            Codec: CodecProvenance.Create("fo-dicom.Codecs", "6.0.0", new string('a', 40)),
            GeneratorSourceSha256: new string('b', 64),
            Fixtures: new[]
            {
                new ReferenceFixtureArtifacts(
                    new ReferenceImageMetadata(
                        "sample-01", 512, 512, 1, 8, 8, false, "MONOCHROME2", 1),
                    new ReferenceSourceArtifacts(
                        "sources/sample-01.dcm",
                        new[] { "sources/frames/sample-01-frame-0000.raw" },
                        new[] { "sources/frames/sample-01-frame-0000.raw" }),
                    new[]
                    {
                        new ReferenceSyntaxArtifacts(
                            "htj2k-lossy",
                            "1.2.840.10008.1.2.4.203",
                            false,
                            "fo-encoded/sample-01-htj2k-lossy.dcm",
                            new[] { "fo-encoded/frames/sample-01-htj2k-lossy-frame-0000.j2c" },
                            "decoded/fo-from-fo/sample-01-htj2k-lossy.dcm",
                            new[] { "decoded/fo-from-fo/sample-01-htj2k-lossy-frame-0000.raw" })
                    })
            },
            Artifacts: new[]
            {
                ArtifactDigest.Create("fo-encoded/frames/sample-01-htj2k-lossy-frame-0000.j2c", new byte[] { 1 })
            });

        var json = ReferenceManifestJson.Serialize(manifest);

        StringAssert.Contains(json, "\"managedVersion\": \"6.0.0\"");
        StringAssert.Contains(json, "\"transferSyntaxUid\": \"1.2.840.10008.1.2.4.203\"");
        StringAssert.Contains(json, "\"encodedFrames\"");
        StringAssert.Contains(json, "\"sha256\"");
    }

    [TestMethod]
    public void GeneratorProducesAllThreeHtJ2kSyntaxesFromRealDicom()
    {
        var repositoryRoot = FindRepositoryRoot();
        var input = Path.Combine(
            repositoryRoot,
            "cmd", "dicom-interop-validation", "fixtures", "sample-01.dcm");
        var output = Path.Combine(Path.GetTempPath(), Guid.NewGuid().ToString("N"));
        try
        {
            var manifest = ReferenceBundleGenerator.Generate(new GeneratorOptions(
                input,
                output,
                "80d8103d394aeed8ce70141c742d7d53620ef90e"));
            var fixture = manifest.Fixtures.Single();

            CollectionAssert.AreEquivalent(
                new[]
                {
                    "1.2.840.10008.1.2.4.201",
                    "1.2.840.10008.1.2.4.202",
                    "1.2.840.10008.1.2.4.203"
                },
                fixture.Syntaxes.Select(syntax => syntax.TransferSyntaxUid).ToArray());
            Assert.IsTrue(File.Exists(Path.Combine(output, "manifest.json")));
            Assert.AreEqual("sources/sample-01.dcm", fixture.Source.Dicom);
            Assert.AreEqual(1, fixture.Source.Frames.Count);
            Assert.IsTrue(manifest.Artifacts.Count >= 14);

            var lossy = fixture.Syntaxes.Single(syntax => syntax.TransferSyntaxUid.EndsWith(".203"));
            Assert.IsFalse(lossy.Lossless);
            foreach (var framePath in lossy.EncodedFrames)
            {
                var frame = File.ReadAllBytes(Path.Combine(output, framePath.Replace('/', Path.DirectorySeparatorChar)));
                CollectionAssert.AreEqual(new byte[] { 0xff, 0xd9 }, frame[^2..]);
            }
        }
        finally
        {
            if (Directory.Exists(output))
            {
                Directory.Delete(output, recursive: true);
            }
        }
    }

    [TestMethod]
    public void GeneratorProducesEveryInputInOneOrderedManifest()
    {
        var repositoryRoot = FindRepositoryRoot();
        var temporaryRoot = Path.Combine(Path.GetTempPath(), Guid.NewGuid().ToString("N"));
        Directory.CreateDirectory(temporaryRoot);
        var source = Path.Combine(
            repositoryRoot,
            "cmd", "dicom-interop-validation", "fixtures", "sample-01.dcm");
        var inputs = new[]
        {
            Path.Combine(temporaryRoot, "fixture-b.dcm"),
            Path.Combine(temporaryRoot, "fixture-a.dcm")
        };
        File.Copy(source, inputs[0]);
        File.Copy(source, inputs[1]);
        var output = Path.Combine(temporaryRoot, "bundle");
        try
        {
            var manifest = ReferenceBundleGenerator.Generate(new GeneratorOptions(
                inputs,
                output,
                "80d8103d394aeed8ce70141c742d7d53620ef90e"));

            Assert.AreEqual(2, manifest.SchemaVersion);
            CollectionAssert.AreEqual(
                new[] { "fixture-b", "fixture-a" },
                manifest.Fixtures.Select(fixture => fixture.Image.Name).ToArray());
            Assert.IsTrue(manifest.Fixtures.All(fixture => fixture.Syntaxes.Count == 3));
        }
        finally
        {
            if (Directory.Exists(temporaryRoot))
            {
                Directory.Delete(temporaryRoot, recursive: true);
            }
        }
    }

    [TestMethod]
    public void FixtureMatrixWriterCreatesMissingAcceptedRangeSources()
    {
        var output = Path.Combine(Path.GetTempPath(), Guid.NewGuid().ToString("N"));
        try
        {
            var paths = FixtureMatrixSourceWriter.Write(output);

            var expected = new[]
            {
                new { Name = "matrix-mono-u8.dcm", Samples = 1, Bits = 8, Photo = "MONOCHROME2", Frames = 1, FrameBytes = 96 * 80 },
                new { Name = "matrix-mono-u16-multiframe.dcm", Samples = 1, Bits = 16, Photo = "MONOCHROME2", Frames = 3, FrameBytes = 96 * 80 * 2 },
                new { Name = "matrix-rgb-u8.dcm", Samples = 3, Bits = 8, Photo = "RGB", Frames = 1, FrameBytes = 96 * 80 * 3 },
                new { Name = "matrix-rgb-u16.dcm", Samples = 3, Bits = 16, Photo = "RGB", Frames = 1, FrameBytes = 96 * 80 * 3 * 2 },
                new { Name = "matrix-ybr-full-u8.dcm", Samples = 3, Bits = 8, Photo = "YBR_FULL", Frames = 1, FrameBytes = 96 * 80 * 3 },
                new { Name = "matrix-ybr-full-422-u8.dcm", Samples = 3, Bits = 8, Photo = "YBR_FULL_422", Frames = 1, FrameBytes = 96 * 80 * 2 }
            };
            CollectionAssert.AreEqual(
                expected.Select(item => item.Name).ToArray(),
                paths.Select(Path.GetFileName).ToArray());

            foreach (var item in expected)
            {
                var file = DicomFile.Open(Path.Combine(output, item.Name), FileReadOption.ReadAll);
                var pixels = DicomPixelData.Create(file.Dataset);
                Assert.AreEqual(96, pixels.Width, item.Name);
                Assert.AreEqual(80, pixels.Height, item.Name);
                Assert.AreEqual(item.Samples, pixels.SamplesPerPixel, item.Name);
                Assert.AreEqual(item.Bits, pixels.BitsAllocated, item.Name);
                Assert.AreEqual(item.Bits, pixels.BitsStored, item.Name);
                Assert.AreEqual(PixelRepresentation.Unsigned, pixels.PixelRepresentation, item.Name);
                Assert.AreEqual(item.Photo, pixels.PhotometricInterpretation.Value, item.Name);
                Assert.AreEqual(item.Frames, pixels.NumberOfFrames, item.Name);
                for (var frame = 0; frame < item.Frames; frame++)
                {
                    Assert.AreEqual(item.FrameBytes, pixels.GetFrame(frame).Size, $"{item.Name} frame {frame}");
                }
            }
        }
        finally
        {
            if (Directory.Exists(output))
            {
                Directory.Delete(output, recursive: true);
            }
        }
    }

    [TestMethod]
    public void ProgramWritesMatrixSourcesThroughExplicitLocalCommand()
    {
        var output = Path.Combine(Path.GetTempPath(), Guid.NewGuid().ToString("N"));
        try
        {
            var exitCode = Program.Main(new[] { "--write-matrix-sources", output });

            Assert.AreEqual(0, exitCode);
            Assert.AreEqual(6, Directory.GetFiles(output, "*.dcm").Length);
        }
        finally
        {
            if (Directory.Exists(output))
            {
                Directory.Delete(output, recursive: true);
            }
        }
    }

    [TestMethod]
    public void GeneratorDecodesYbrSourcesAfterNativeEncoderConvertsThemToRgb()
    {
        var temporaryRoot = Path.Combine(Path.GetTempPath(), Guid.NewGuid().ToString("N"));
        var sourceDirectory = Path.Combine(temporaryRoot, "sources");
        var output = Path.Combine(temporaryRoot, "bundle");
        try
        {
            var inputs = FixtureMatrixSourceWriter.Write(sourceDirectory)
                .Where(path => Path.GetFileName(path).StartsWith("matrix-ybr-", StringComparison.Ordinal))
                .ToArray();

            var manifest = ReferenceBundleGenerator.Generate(new GeneratorOptions(
                inputs,
                output,
                "80d8103d394aeed8ce70141c742d7d53620ef90e"));

            Assert.AreEqual(2, manifest.Fixtures.Count);
            foreach (var fixture in manifest.Fixtures)
            {
                Assert.IsTrue(fixture.Image.PhotometricInterpretation.StartsWith("YBR_", StringComparison.Ordinal));
                Assert.AreEqual(fixture.Image.FrameCount, fixture.Source.EncoderInputFrames.Count);
                var encoderInputPath = fixture.Source.EncoderInputFrames.Single();
                var encoderInput = File.ReadAllBytes(Path.Combine(
                    output,
                    encoderInputPath.Replace('/', Path.DirectorySeparatorChar)));
                Assert.AreEqual(96 * 80 * 3, encoderInput.Length);
                if (fixture.Image.PhotometricInterpretation == "YBR_FULL")
                {
                    CollectionAssert.AreEqual(new byte[] { 54, 26, 4 }, encoderInput[..3]);
                }
                foreach (var syntax in fixture.Syntaxes)
                {
                    var decodedFrame = manifest.Artifacts.Single(
                        artifact => artifact.Path == syntax.DecodedFrames.Single());
                    Assert.AreEqual(96 * 80 * 3, decodedFrame.Length);
                }
            }
        }
        finally
        {
            if (Directory.Exists(temporaryRoot))
            {
                Directory.Delete(temporaryRoot, recursive: true);
            }
        }
    }

    [TestMethod]
    public void GoBundleDecoderProducesEveryFoFromGoArtifactFromSavedGoDicom()
    {
        var repositoryRoot = FindRepositoryRoot();
        var sourceBundle = Path.Combine(repositoryRoot, "test-data", "htj2k", "interop-v1");
        var temporaryRoot = Path.Combine(Path.GetTempPath(), Guid.NewGuid().ToString("N"));
        var bundle = Path.Combine(temporaryRoot, "interop-v1");
        try
        {
            CopyDirectory(sourceBundle, bundle);
            var original = ReferenceManifestJson.Deserialize(
                File.ReadAllText(Path.Combine(bundle, "manifest.json")));
            var updated = GoBundleDecoder.Decode(new GoBundleDecodeOptions(
                bundle,
                original.Codec.SourceCommit));

            Assert.AreEqual(original.Fixtures.Count, updated.Fixtures.Count);
            foreach (var fixture in updated.Fixtures)
            {
                foreach (var syntax in fixture.Syntaxes)
                {
                    Assert.IsFalse(string.IsNullOrWhiteSpace(syntax.FoFromGoDicom));
                    Assert.AreEqual(fixture.Image.FrameCount, syntax.FoFromGoFrames?.Count);
                    for (var frame = 0; frame < fixture.Image.FrameCount; frame++)
                    {
                        var foFromGo = File.ReadAllBytes(Path.Combine(
                            bundle,
                            syntax.FoFromGoFrames![frame].Replace('/', Path.DirectorySeparatorChar)));
                        var foFromFo = File.ReadAllBytes(Path.Combine(
                            bundle,
                            syntax.DecodedFrames[frame].Replace('/', Path.DirectorySeparatorChar)));
                        CollectionAssert.AreEqual(
                            foFromFo,
                            foFromGo,
                            $"{fixture.Image.Name}/{syntax.Name}/frame-{frame:D4}");
                        if (syntax.Lossless
                            && fixture.Image.PhotometricInterpretation != "YBR_FULL"
                            && fixture.Image.PhotometricInterpretation != "YBR_FULL_422"
                            && (fixture.Image.BitsAllocated != 16 || fixture.Image.SamplesPerPixel == 1))
                        {
                            var source = File.ReadAllBytes(Path.Combine(
                                bundle,
                                fixture.Source.Frames[frame].Replace('/', Path.DirectorySeparatorChar)));
                            CollectionAssert.AreEqual(
                                source,
                                foFromGo,
                                $"{fixture.Image.Name}/{syntax.Name}/frame-{frame:D4}/source");
                        }
                    }
                }
            }
        }
        finally
        {
            if (Directory.Exists(temporaryRoot))
            {
                Directory.Delete(temporaryRoot, recursive: true);
            }
        }
    }

    private static void CopyDirectory(string source, string destination)
    {
        foreach (var directory in Directory.EnumerateDirectories(source, "*", SearchOption.AllDirectories))
        {
            Directory.CreateDirectory(Path.Combine(destination, Path.GetRelativePath(source, directory)));
        }
        Directory.CreateDirectory(destination);
        foreach (var file in Directory.EnumerateFiles(source, "*", SearchOption.AllDirectories))
        {
            var destinationPath = Path.Combine(destination, Path.GetRelativePath(source, file));
            Directory.CreateDirectory(Path.GetDirectoryName(destinationPath)!);
            File.Copy(file, destinationPath);
        }
    }

    private static string FindRepositoryRoot()
    {
        var directory = new DirectoryInfo(AppContext.BaseDirectory);
        while (directory is not null && !File.Exists(Path.Combine(directory.FullName, "go.mod")))
        {
            directory = directory.Parent;
        }
        return directory?.FullName
            ?? throw new InvalidOperationException("Cannot locate repository root from test output directory.");
    }
}

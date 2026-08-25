using System.Buffers.Binary;
using FellowOakDicom;
using FellowOakDicom.Imaging;
using FellowOakDicom.IO.Buffer;

namespace FoDicomReferenceGenerator;

public static class FixtureMatrixSourceWriter
{
    private const int Width = 96;
    private const int Height = 80;

    private sealed record Definition(
        string Name,
        int Samples,
        int Bits,
        string Photometric,
        int FrameCount,
        Func<int, byte[]> CreateFrame);

    private static readonly Definition[] Definitions =
    {
        new("matrix-mono-u8", 1, 8, "MONOCHROME2", 1, CreateMono8),
        new("matrix-mono-u16-multiframe", 1, 16, "MONOCHROME2", 3, CreateMono16),
        new("matrix-rgb-u8", 3, 8, "RGB", 1, CreateRgb8),
        new("matrix-rgb-u16", 3, 16, "RGB", 1, CreateRgb16),
        new("matrix-ybr-full-u8", 3, 8, "YBR_FULL", 1, CreateYbrFull8),
        new("matrix-ybr-full-422-u8", 3, 8, "YBR_FULL_422", 1, CreateYbrFull4228)
    };

    public static IReadOnlyList<string> Write(string outputDirectory)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(outputDirectory);
        if (Directory.Exists(outputDirectory)
            && Directory.EnumerateFileSystemEntries(outputDirectory).Any())
        {
            throw new InvalidOperationException(
                $"Fixture output directory '{outputDirectory}' must be empty.");
        }

        Directory.CreateDirectory(outputDirectory);
        var paths = new List<string>(Definitions.Length);
        for (var index = 0; index < Definitions.Length; index++)
        {
            var definition = Definitions[index];
            var path = Path.Combine(outputDirectory, definition.Name + ".dcm");
            CreateFile(definition, index + 1).Save(path);
            paths.Add(path);
        }
        return paths;
    }

    private static DicomFile CreateFile(Definition definition, int identifier)
    {
        var dataset = new DicomDataset(DicomTransferSyntax.ExplicitVRLittleEndian);
        dataset.AddOrUpdate(DicomTag.SOPClassUID, DicomUID.SecondaryCaptureImageStorage);
        dataset.AddOrUpdate(
            DicomTag.SOPInstanceUID,
            $"1.2.826.0.1.3680043.10.543.20260825.{identifier}");
        dataset.AddOrUpdate(DicomTag.StudyInstanceUID, "1.2.826.0.1.3680043.10.543.20260825.100");
        dataset.AddOrUpdate(DicomTag.SeriesInstanceUID, "1.2.826.0.1.3680043.10.543.20260825.200");
        dataset.AddOrUpdate(DicomTag.Modality, "OT");
        dataset.AddOrUpdate(DicomTag.PatientID, "HTJ2K-MATRIX");
        dataset.AddOrUpdate(DicomTag.Rows, (ushort)Height);
        dataset.AddOrUpdate(DicomTag.Columns, (ushort)Width);
        dataset.AddOrUpdate(DicomTag.SamplesPerPixel, (ushort)definition.Samples);
        dataset.AddOrUpdate(DicomTag.PhotometricInterpretation, definition.Photometric);
        if (definition.Samples > 1)
        {
            dataset.AddOrUpdate(DicomTag.PlanarConfiguration, (ushort)0);
        }
        dataset.AddOrUpdate(DicomTag.BitsAllocated, (ushort)definition.Bits);
        dataset.AddOrUpdate(DicomTag.BitsStored, (ushort)definition.Bits);
        dataset.AddOrUpdate(DicomTag.HighBit, (ushort)(definition.Bits - 1));
        dataset.AddOrUpdate(DicomTag.PixelRepresentation, (ushort)0);

        var pixelData = DicomPixelData.Create(dataset, true);
        for (var frame = 0; frame < definition.FrameCount; frame++)
        {
            pixelData.AddFrame(new MemoryByteBuffer(definition.CreateFrame(frame)));
        }
        return new DicomFile(dataset);
    }

    private static byte[] CreateMono8(int frameIndex)
    {
        var frame = new byte[Width * Height];
        for (var y = 0; y < Height; y++)
        {
            for (var x = 0; x < Width; x++)
            {
                frame[y * Width + x] = (byte)(24 + frameIndex * 3 + (x / 8) * 7 + (y / 8) * 5);
            }
        }
        return frame;
    }

    private static byte[] CreateMono16(int frameIndex)
    {
        var frame = new byte[Width * Height * 2];
        for (var y = 0; y < Height; y++)
        {
            for (var x = 0; x < Width; x++)
            {
                var value = (ushort)(frameIndex * 4096 + (x / 4) * 113 + (y / 4) * 79);
                BinaryPrimitives.WriteUInt16LittleEndian(frame.AsSpan((y * Width + x) * 2), value);
            }
        }
        return frame;
    }

    private static byte[] CreateRgb8(int frameIndex)
    {
        var frame = new byte[Width * Height * 3];
        for (var y = 0; y < Height; y++)
        {
            for (var x = 0; x < Width; x++)
            {
                var offset = (y * Width + x) * 3;
                frame[offset] = (byte)(16 + frameIndex + (x / 8) * 12);
                frame[offset + 1] = (byte)(24 + (y / 8) * 14);
                frame[offset + 2] = (byte)(32 + ((x + y) / 8) * 6);
            }
        }
        return frame;
    }

    private static byte[] CreateRgb16(int frameIndex)
    {
        var frame = new byte[Width * Height * 3 * 2];
        for (var y = 0; y < Height; y++)
        {
            for (var x = 0; x < Width; x++)
            {
                var offset = (y * Width + x) * 6;
                BinaryPrimitives.WriteUInt16LittleEndian(frame.AsSpan(offset), (ushort)(frameIndex * 256 + x * 512));
                BinaryPrimitives.WriteUInt16LittleEndian(frame.AsSpan(offset + 2), (ushort)(y * 640));
                BinaryPrimitives.WriteUInt16LittleEndian(frame.AsSpan(offset + 4), (ushort)((x + y) * 288));
            }
        }
        return frame;
    }

    private static byte[] CreateYbrFull8(int frameIndex)
    {
        var frame = new byte[Width * Height * 3];
        for (var y = 0; y < Height; y++)
        {
            for (var x = 0; x < Width; x++)
            {
                var offset = (y * Width + x) * 3;
                frame[offset] = (byte)(32 + frameIndex + (x / 8) * 8 + (y / 8) * 4);
                frame[offset + 1] = (byte)(112 + (y / 16) * 4);
                frame[offset + 2] = (byte)(144 - (x / 16) * 4);
            }
        }
        return frame;
    }

    private static byte[] CreateYbrFull4228(int frameIndex)
    {
        var frame = new byte[Width * Height * 2];
        for (var y = 0; y < Height; y++)
        {
            for (var x = 0; x < Width; x += 2)
            {
                var offset = (y * Width + x) * 2;
                frame[offset] = (byte)(40 + frameIndex + (x / 8) * 7 + (y / 8) * 3);
                frame[offset + 1] = (byte)(42 + frameIndex + (x / 8) * 7 + (y / 8) * 3);
                frame[offset + 2] = (byte)(116 + (y / 16) * 3);
                frame[offset + 3] = (byte)(140 - (x / 16) * 3);
            }
        }
        return frame;
    }
}

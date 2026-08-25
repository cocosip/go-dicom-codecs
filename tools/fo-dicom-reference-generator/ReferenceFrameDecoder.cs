using FellowOakDicom;
using FellowOakDicom.Imaging;
using FellowOakDicom.Imaging.Codec;
using FellowOakDicom.IO.Buffer;

namespace FoDicomReferenceGenerator;

public static class ReferenceFrameDecoder
{
    public static DicomFile Decode(DicomFile encoded)
    {
        ArgumentNullException.ThrowIfNull(encoded);
        if (!encoded.Dataset.InternalTransferSyntax.IsEncapsulated)
        {
            throw new InvalidDataException("Reference frame decoding requires encapsulated pixel data.");
        }

        var encodedPixelData = DicomPixelData.Create(encoded.Dataset);
        var transcoder = new DicomTranscoder(
            encoded.Dataset.InternalTransferSyntax,
            DicomTransferSyntax.ExplicitVRLittleEndian);
        var decodedDataset = new DicomDataset((IEnumerable<DicomItem>)encoded.Dataset);
        var decodedPixelData = DicomPixelData.Create(decodedDataset, true);
        for (var frame = 0; frame < encodedPixelData.NumberOfFrames; frame++)
        {
            var singleFrameDataset = new DicomDataset((IEnumerable<DicomItem>)encoded.Dataset);
            var singleFramePixelData = DicomPixelData.Create(singleFrameDataset, true);
            singleFramePixelData.AddFrame(encodedPixelData.GetFrame(frame));
            if (singleFramePixelData.NumberOfFrames != 1)
            {
                throw new InvalidDataException("Single-frame decode input did not contain exactly one frame.");
            }
            var singleFrameDecoded = transcoder.Transcode(new DicomFile(singleFrameDataset));
            var decodedFrame = DicomPixelData.Create(singleFrameDecoded.Dataset).GetFrame(0);
            decodedPixelData.AddFrame(new MemoryByteBuffer(decodedFrame.Data.ToArray()));
        }
        return new DicomFile(decodedDataset);
    }
}

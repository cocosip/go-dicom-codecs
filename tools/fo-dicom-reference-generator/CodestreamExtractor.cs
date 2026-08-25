namespace FoDicomReferenceGenerator;

public static class CodestreamExtractor
{
    private const byte MarkerPrefix = 0xff;
    private const byte Soc = 0x4f;
    private const byte Eoc = 0xd9;

    public static byte[] Extract(ReadOnlySpan<byte> frame)
    {
        if (frame.Length < 4 || frame[0] != MarkerPrefix || frame[1] != Soc)
        {
            throw new InvalidDataException("HTJ2K frame does not start with the SOC marker.");
        }

        for (var index = 2; index + 1 < frame.Length; index++)
        {
            if (frame[index] == MarkerPrefix && frame[index + 1] == Eoc)
            {
                return frame[..(index + 2)].ToArray();
            }
        }

        throw new InvalidDataException("HTJ2K frame does not contain an EOC marker.");
    }
}

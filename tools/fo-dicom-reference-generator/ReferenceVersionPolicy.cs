namespace FoDicomReferenceGenerator;

public static class ReferenceVersionPolicy
{
    public const string AcceptedRange = "[6.0.0-beta1,7.0.0)";

    public static bool IsAccepted(string? value)
    {
        if (string.IsNullOrWhiteSpace(value))
        {
            return false;
        }

        var withoutBuild = value.Split('+', 2)[0];
        var parts = withoutBuild.Split('-', 2);
        if (!Version.TryParse(parts[0], out var version) || version.Major != 6)
        {
            return false;
        }

        if (version.Minor > 0 || version.Build > 0 || parts.Length == 1)
        {
            return true;
        }

        var prerelease = parts[1];
        if (prerelease.StartsWith("rc", StringComparison.OrdinalIgnoreCase))
        {
            return true;
        }
        if (!prerelease.StartsWith("beta", StringComparison.OrdinalIgnoreCase))
        {
            return false;
        }

        var suffix = prerelease["beta".Length..].TrimStart('.', '-');
        return int.TryParse(suffix, out var betaNumber) && betaNumber >= 1;
    }

    public static void RequireAccepted(string value)
    {
        if (!IsAccepted(value))
        {
            throw new InvalidOperationException(
                $"Loaded fo-dicom.Codecs version '{value}' is outside {AcceptedRange}.");
        }
    }
}

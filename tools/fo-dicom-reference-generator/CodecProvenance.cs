using System.Reflection;

namespace FoDicomReferenceGenerator;

public sealed record CodecProvenance(
    string AssemblyName,
    string ManagedVersion,
    string SourceCommit)
{
    public static CodecProvenance FromAssembly(Assembly assembly, string sourceCommit)
    {
        ArgumentNullException.ThrowIfNull(assembly);

        var name = assembly.GetName();
        var informationalVersion = assembly
            .GetCustomAttribute<AssemblyInformationalVersionAttribute>()?
            .InformationalVersion;
        var version = informationalVersion ?? name.Version?.ToString();
        if (string.IsNullOrWhiteSpace(version))
        {
            throw new InvalidOperationException(
                $"Loaded assembly '{name.Name}' does not expose a managed version.");
        }

        return Create(name.Name ?? "unknown", version, sourceCommit);
    }

    public static CodecProvenance Create(
        string assemblyName,
        string managedVersion,
        string sourceCommit)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(assemblyName);
        ArgumentException.ThrowIfNullOrWhiteSpace(managedVersion);
        ArgumentException.ThrowIfNullOrWhiteSpace(sourceCommit);

        ReferenceVersionPolicy.RequireAccepted(managedVersion);
        return new CodecProvenance(assemblyName, managedVersion, sourceCommit);
    }
}

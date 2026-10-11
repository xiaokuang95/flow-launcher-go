// The reference's URL validation, copied unchanged from
// Plugins/Flow.Launcher.Plugin.Url/Main.cs so that this port can be checked
// against it. Only the parts that read the plugin's settings are left out:
// the scheme a URL typed without one is opened with, which is "http" unless
// the user changed it.
using System;
using System.Linq;
using System.Net;
using System.Net.Sockets;

namespace UrlReference;

public static class UrlValidator
{
    // Schemes requiring full host validation: domain with TLD, IP address, or localhost
    private static readonly string[] HostValidatedSchemes = ["http", "https"];

    // Chromium browser schemes accepting both :// and : forms (e.g. chrome://settings, chrome:settings)
    private static readonly string[] ChromiumSchemes = ["chrome-extension", "chrome", "brave", "edge", "opera", "vivaldi"];

    // Schemes using :// that are validated by scheme recognition alone
    private static readonly string[] NonHostValidatedDoubleSlashSchemes = [.. ChromiumSchemes, "file", "moz-extension"];

    // Schemes using colon-only syntax (e.g. about:blank, chrome:settings)
    private static readonly string[] ColonOnlySchemes = [.. ChromiumSchemes, "about", "data"];

    // All :// schemes
    private static readonly string[] AllDoubleSlashSchemes = [.. HostValidatedSchemes, .. NonHostValidatedDoubleSlashSchemes];

    public static bool IsURL(string raw)
    {
        if (string.IsNullOrWhiteSpace(raw))
            return false;

        var input = raw.Trim();

        // Exclude numbers (e.g. 1.2345)
        if (decimal.TryParse(input, System.Globalization.NumberStyles.Any, System.Globalization.CultureInfo.InvariantCulture, out _))
            return false;

        if (TryMatchBareIP(input, out var bareIPValid))
            return bareIPValid;

        if (TryMatchColonScheme(input, out var colonValid))
            return colonValid;

        var urlToValidate = AllDoubleSlashSchemes.Any(s => input.StartsWith(s + "://", StringComparison.OrdinalIgnoreCase))
            ? input
            : "http" + "://" + input;

        if (!Uri.TryCreate(urlToValidate, UriKind.Absolute, out var uri))
            return false;

        if (!AllDoubleSlashSchemes.Any(scheme => uri.Scheme == scheme))
            return false;

        if (TryMatchNonHostScheme(uri, input, out var nonHostValid))
            return nonHostValid;

        return ValidateSchemeHost(uri);
    }

    private static bool TryMatchBareIP(string input, out bool isValid)
    {
        var ipPart = input.Split('/', '?', '#')[0];
        if (!IPEndPoint.TryParse(ipPart, out var endpoint))
        {
            isValid = false;
            return false;
        }

        switch (endpoint.AddressFamily)
        {
            case AddressFamily.InterNetwork:
                if (ipPart.Split('.').Length != 4)
                {
                    isValid = false;
                    return true;
                }
                isValid = !endpoint.Address.Equals(IPAddress.Any);
                return true;
            case AddressFamily.InterNetworkV6:
                if (input.Contains('/') || input.Contains('?') || input.Contains('#'))
                {
                    var bracketStart = input.IndexOf('[');
                    var bracketEnd = input.IndexOf(']');
                    if (bracketStart == -1 || bracketEnd == -1 || bracketStart > bracketEnd)
                    {
                        isValid = false;
                        return true;
                    }
                }
                isValid = !endpoint.Address.Equals(IPAddress.IPv6Any);
                return true;
        }

        isValid = true;
        return true;
    }

    private static bool TryMatchColonScheme(string input, out bool isValid)
    {
        int colonIndex = input.IndexOf(':');

        bool hasColonPrefix = colonIndex > 0;

        bool isKnownScheme = hasColonPrefix
            && ColonOnlySchemes.Any(s => input[..colonIndex].Equals(s, StringComparison.OrdinalIgnoreCase));

        bool isColonOnlySyntax = isKnownScheme
            && !input[(colonIndex + 1)..].StartsWith("//");

        if (!isColonOnlySyntax)
        {
            isValid = false;
            return false;
        }

        var content = input[(colonIndex + 1)..];
        bool hasContent = content.Length > 0;
        bool hasNoWhitespace = !content.Any(char.IsWhiteSpace);

        isValid = hasContent && hasNoWhitespace;
        return true;
    }

    private static bool TryMatchNonHostScheme(Uri uri, string input, out bool isValid)
    {
        if (!NonHostValidatedDoubleSlashSchemes.Any(scheme => uri.Scheme == scheme))
        {
            isValid = false;
            return false;
        }

        string schemePrefix = uri.Scheme + "://";
        bool hasContent = input.Length > schemePrefix.Length;
        isValid = hasContent;
        return true;
    }

    private static bool ValidateSchemeHost(Uri uri)
    {
        var host = uri.Host;

        if (host.Equals("localhost", StringComparison.OrdinalIgnoreCase))
            return true;

        if (IPEndPoint.TryParse(host, out var endpoint))
            return !endpoint.Address.Equals(IPAddress.Any) && !endpoint.Address.Equals(IPAddress.IPv6Any);

        var parts = host.Split('.');
        if (parts.Length < 2 || parts.Any(string.IsNullOrEmpty))
            return false;

        var tld = parts[^1];
        return tld.Length >= 2 && tld.All(char.IsLetterOrDigit);
    }
}

// oracle runs parts of the reference implementation over cases given on
// stdin, so that the Go port can be checked against the code it copies
// rather than against a reading of it.
//
// Each line of stdin is a JSON object with a "kind":
//
//	{"kind":"match","query":"chr","compare":"Chrome","precision":50,"ignoreAccents":false}
//	{"kind":"resultHash","title":"a","subTitle":"b","recordKey":""}
//	{"kind":"queryResultHash","actionKeyword":"f","search":"a","title":"a","subTitle":"b"}
//
// and each line of stdout is the answer.
//
// Both streams are UTF-8 whatever the console's code page is, which matters
// on Windows where the console would otherwise read the cases in its own
// encoding.
using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Globalization;
using System.Text;
using System.Text.Json;
using Mages.Core;
using Flow.Launcher.Infrastructure;
using Flow.Launcher.Infrastructure.UserSettings;
using Flow.Launcher.Plugin;
using Flow.Launcher.Plugin.SharedModels;

var utf8 = new UTF8Encoding(false);
using var input = new StreamReader(Console.OpenStandardInput(), utf8);
using var output = new StreamWriter(Console.OpenStandardOutput(), utf8) { AutoFlush = true };

var options = new JsonSerializerOptions { Encoder = System.Text.Encodings.Web.JavaScriptEncoder.UnsafeRelaxedJsonEscaping };

string line;
var index = -1;
while ((line = input.ReadLine()) != null)
{
    if (line.Length == 0) continue;
    index++;

    JsonDocument doc;
    try
    {
        doc = JsonDocument.Parse(line);
    }
    catch (Exception e)
    {
        output.WriteLine(JsonSerializer.Serialize(new { index, error = "parse: " + e.Message }, options));
        continue;
    }

    using (doc)
    {
        var root = doc.RootElement;
        var kind = root.TryGetProperty("kind", out var k) ? k.GetString() : "match";

        try
        {
            switch (kind)
            {
                case "match":
                    output.WriteLine(JsonSerializer.Serialize(Match(root), options));
                    break;
                case "culture":
                    output.WriteLine(JsonSerializer.Serialize(Culture(root), options));
                    break;
                case "toDecimal":
                    output.WriteLine(JsonSerializer.Serialize(new
                    {
                        index,
                        text = Convert.ToDecimal(root.GetProperty("value").GetDouble()).ToString(CultureInfo.InvariantCulture),
                    }, options));
                    break;
                case "decimal":
                    output.WriteLine(JsonSerializer.Serialize(new
                    {
                        index,
                        text = Round(root),
                    }, options));
                    break;
                case "isURL":
                    output.WriteLine(JsonSerializer.Serialize(new
                    {
                        index,
                        isURL = UrlReference.UrlValidator.IsURL(Text(root, "input") ?? ""),
                    }, options));
                    break;
                case "mages":
                    output.WriteLine(JsonSerializer.Serialize(Mages(root), options));
                    break;
                case "resultHash":
                    output.WriteLine(JsonSerializer.Serialize(new
                    {
                        index,
                        hash = ResultHash(root),
                    }, options));
                    break;
                case "queryResultHash":
                    output.WriteLine(JsonSerializer.Serialize(new
                    {
                        index,
                        hash = QueryResultHash(root),
                    }, options));
                    break;
                default:
                    output.WriteLine(JsonSerializer.Serialize(new { index, error = "unknown kind " + kind }, options));
                    break;
            }
        }
        catch (Exception e)
        {
            output.WriteLine(JsonSerializer.Serialize(new { index, error = e.GetType().Name + ": " + e.Message }, options));
        }
    }
}

static string? Text(JsonElement root, string name) =>
    root.TryGetProperty(name, out var value) && value.ValueKind != JsonValueKind.Null ? value.GetString() : null;

static Result MakeResult(JsonElement root)
{
    var result = new Result
    {
        Title = Text(root, "title"),
        SubTitle = Text(root, "subTitle") ?? string.Empty,
        RecordKey = Text(root, "recordKey"),
    };
    return result;
}

static object Match(JsonElement root)
{
    var query = Text(root, "query");
    var compare = Text(root, "compare");
    var precision = root.TryGetProperty("precision", out var p) ? (SearchPrecisionScore)p.GetInt32() : SearchPrecisionScore.Regular;
    var ignoreAccents = root.TryGetProperty("ignoreAccents", out var a) && a.GetBoolean();

    var settings = new Settings { IgnoreAccents = ignoreAccents };
    var matcher = new StringMatcher(null, settings) { UserSettingSearchPrecision = precision };
    var result = matcher.FuzzyMatch(query, compare);

    return new
    {
        success = result.Success,
        rawScore = result.RawScore,
        score = result.Score,
        met = result.IsSearchPrecisionScoreMet(),
        matchData = result.MatchData ?? new List<int>()
    };
}

// Culture is the number format the desktop's culture uses, which is what
// the calculator reads to separate decimals and groups.
static object Culture(JsonElement root)
{
    var name = Text(root, "name") ?? "";
    var culture = CultureInfo.GetCultureInfo(name);
    return new
    {
        index = -1,
        name = culture.Name,
        decimalSeparator = culture.NumberFormat.NumberDecimalSeparator,
        groupSeparator = culture.NumberFormat.NumberGroupSeparator,
        groupSizes = culture.NumberFormat.NumberGroupSizes,
    };
}

// Round is what the calculator does to a value before showing it: the
// double becomes a decimal, which carries the number of decimal places it
// was written with, and is rounded to that many, at most places.
static string Round(JsonElement root)
{
    var value = root.GetProperty("value").GetDouble();
    var places = root.TryGetProperty("places", out var p) ? p.GetInt32() : 10;

    decimal asDecimal;
    try
    {
        asDecimal = Convert.ToDecimal(value);
    }
    catch (Exception)
    {
        // Infinity and NaN have no decimal form.
        return "";
    }

    return Math.Round(asDecimal, places, MidpointRounding.AwayFromZero)
        .ToString(CultureInfo.InvariantCulture);
}

static object Mages(JsonElement root)
{
    var expression = Text(root, "expression") ?? string.Empty;
    var engine = new Engine(new Configuration
    {
        Scope = new Dictionary<string, object> { { "e", Math.E } },
    });

    object result;
    try
    {
        result = engine.Interpret(expression);
    }
    catch (Exception e)
    {
        return new { index = -1, error = e.GetType().Name + ": " + e.Message };
    }

    return new
    {
        index = -1,
        text = result?.ToString() ?? "",
        type = result?.GetType().Name ?? "null",
    };
}

static int ResultHash(JsonElement root)
{
    var result = MakeResult(root);
    // The reference's private helpers, reached the way it reaches them: a
    // record's counts are keyed by these.
    var type = typeof(Result);
    var assembly = type.Assembly;
    var holders = assembly.GetTypes().Where(t => t.Name == "UserSelectedRecord").ToList();
    if (holders.Count == 0)
        throw new InvalidOperationException("UserSelectedRecord is not in the same assembly as Result");

    var method = holders[0].GetMethod("GenerateResultHashCode",
        System.Reflection.BindingFlags.NonPublic | System.Reflection.BindingFlags.Static);
    return (int)method.Invoke(null, new object[] { result });
}

static int QueryResultHash(JsonElement root)
{
    var result = MakeResult(root);
    var query = new Query { TrimmedQuery = Text(root, "trimmedQuery") ?? string.Empty };
    // Query's ActionKeyword and Search are init-only, so they are set the
    // way the builder sets them, through reflection on the backing field.
    SetQueryPart(query, "ActionKeyword", Text(root, "actionKeyword") ?? string.Empty);
    SetQueryPart(query, "Search", Text(root, "search") ?? string.Empty);

    var holders = typeof(Result).Assembly.GetTypes().Where(t => t.Name == "UserSelectedRecord").ToList();
    var method = holders[0].GetMethod("GenerateQueryAndResultHashCode",
        System.Reflection.BindingFlags.NonPublic | System.Reflection.BindingFlags.Static);
    return (int)method.Invoke(null, new object[] { query, result });
}

static void SetQueryPart(Query query, string name, string value)
{
    var field = typeof(Query).GetField($"<{name}>k__BackingField",
        System.Reflection.BindingFlags.NonPublic | System.Reflection.BindingFlags.Instance);
    if (field is null) throw new InvalidOperationException($"Query has no {name} backing field");
    field.SetValue(query, value);
}

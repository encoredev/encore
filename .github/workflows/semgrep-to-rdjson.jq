# See https://github.com/reviewdog/reviewdog/tree/master/proto/rdf
{
    source: {
        name: "semgrep",
        url: "https://semgrep.dev/",
    },
    diagnostics: [
        .results[] | {
            code: {
                value: .check_id,
                url: [
                        .extra.metadata.shortlink?,
                        .extra.metadata.source?,
                        .extra."semgrep.dev".rule.url?,
                        "https://github.com/encoredev/encore/blob/main/\(.check_id | gsub("\\."; "/")).yml"
                    ] | map(select(. != null)) | first,
            },
            message: .extra.message,
            location: {
                path: .path,
                range: {
                    start: {
                        line: .start.line,
                        column: .start.col
                    },
                    end: {
                        line: .end.line,
                        column: .end.col
                    },
                },
            },
            severity: .extra.severity,

            # Temporary variable we store to track the fix
            _res: .
        } |
            if ._res.extra.fix then .suggestions = [{
                range: .location.range,
                text: ._res.extra.fix,
            }] else . end |
            del(._res)
    ]
}

# See https://github.com/reviewdog/reviewdog/tree/master/proto/rdf
{
  source: {
    name: "staticcheck",
    url: "https://staticcheck.io"
  },
  message: .message,
  code: {value: .code, url: "https://staticcheck.io/docs/checks#\(.code)"},
  location: {
    path: .location.file,
    range: {
      start: {
        line: .location.line,
        column: .location.column
      }
    }
  },
  severity: ((.severity|ascii_upcase|select(match("ERROR|WARNING|INFO")))//null)
}

import React from 'react'
import { decodeBase64 } from '~lib/base64'
import { ProcessOutput, ProcessStart, ProcessStop } from '~lib/client/client'
import JSONRPCConn, { NotificationMsg } from '~lib/client/jsonrpc'
import parseAnsi, { Chunk } from "~lib/parse-ansi"

interface Props {
  appID: string;
  conn: JSONRPCConn;
}

interface State {
  lines: Chunk[][];
}

export default class AppLogs extends React.Component<Props, State> {
  constructor(props: Props) {
    super(props)
    this.state = {lines: [[]]}
    this.onNotification = this.onNotification.bind(this)
  }

  componentDidMount() {
    this.props.conn.on("notification", this.onNotification)
  }

  componentWillUnmount() {
    this.props.conn.off("notification", this.onNotification)
  }

  onNotification(msg: NotificationMsg) {
    if (msg.method === "process/start") {
      const data = msg.params as ProcessStart
      if (data.appID === this.props.appID) {
        this.setState(state => {
          return {
            lines: [...state.lines, [
              {type: "text", style: {}, value: "Running on "},
              {type: "text", style: {bold: true}, value: `http://localhost:${data.port}`},
            ], []]
          }
        })
      }
    } else if (msg.method === "process/stop") {
      const data = msg.params as ProcessStop
      if (data.appID === this.props.appID) {
        this.setState(state => {
          return {
            lines: [...state.lines, [
              {type: "text", style: {foregroundColor: "red"}, value: "App stopped."},
            ], []]
          }
        })
      }
    } else if (msg.method === "process/output") {
      const data = msg.params as ProcessOutput
      if (data.appID === this.props.appID) {
        let chunks = parseAnsi(decodeBase64(data.output)).chunks as Chunk[]
        let newLines: Chunk[][] = [[]]
        for (const ch of chunks) {
          if (ch.type === "newline") {
            newLines.push([])
          } else if (ch.type === "text") {
            newLines[newLines.length - 1].push(ch)
          }
        }
        this.setState(state => {
          let prev = state.lines.slice(0, -1)
          let curr = state.lines[state.lines.length - 1]
          curr = curr.concat(newLines[0])
          return {
            lines: [...prev, curr, ...newLines.slice(1)],
          }
        })
      }
    }
  }

  render() {
    return (
      <div className="h-full relative text-sm font-mono subpixel-antialiased bg-gray-800 px-5 py-2 leading-normal" style={{lineHeight: 1.675}}>
        <style>{`
          .gray     { color: rgb(228, 218, 199); }
          .green    { color: #B5F4A5; }
          .blue     { color: #93DDFD; }
          .red      { color: #FF8383; }
          .purple   { color: #D9A9FF; }
          .white    { color: #FFFFFF; }
          .darkgray { color: #718096; }
          div {
            font-size: 14px;
            line-height: 1.375;
            tab-size: 4;
          }
        `}</style>
        <div className="h-full font-mono whitespace-pre text-white overflow-x-scroll overflow-y-scroll">
          {this.state.lines.map((line, i) =>
            <div key={i}>
              {line.map((ch, j) =>
                <span key={j} className={chunkStyle(ch)}>{ch.value}</span>
              )}
              {line.length === 0 ? <span>&nbsp;</span> : null}
            </div>
          )}
        </div>
      </div>
    )
  }
}

function chunkStyle(ch: Chunk): string {
  const cls = []
  if (ch.style.bold) { cls.push("font-bold") }
  if (ch.style.italic) { cls.push("italic") }
  if (ch.style.strikethrough) { cls.push("line-through") }
  if (ch.style.underline) { cls.push("underline") }

  const fc = ch.style.foregroundColor
  if (fc) {
    cls.push(
      fc === "gray" ? "text-gray-500" :
      fc === "green" ? "green" :
      fc === "red" ? "red" :
      fc === "blue" ? "blue" :
      fc === "cyan" ? "purple" :
      "text-white"
    )
  }
  return cls.join(" ")
}
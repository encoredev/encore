import React from "react";

import CodeMirror, { EditorConfiguration } from "codemirror";
import styles from "./codemirror-css";
import showHintStyles from "./codemirror-show-hint-css";
import encoreStyles from "./codemirror-encore-css";
import ideaStyles from "./codemirror-idea-css";
import mdStyles from "./codemirror-md-css";
import "codemirror/mode/go/go.js";
import "codemirror/mode/sql/sql.js";
import "codemirror/mode/javascript/javascript.js";
import "codemirror/mode/markdown/markdown.js";
import "codemirror/addon/edit/closebrackets.js";
import "codemirror/addon/edit/matchbrackets.js";
import "codemirror/addon/selection/active-line.js";

import "codemirror/addon/hint/show-hint.js";

export interface TextEdit {
  newText: string;
  range: {
    start: { line: number; character: number };
    end: { line: number; character: number };
  };
}

export const DefaultCfg: EditorConfiguration = {
  theme: "encore",
  mode: "go",
  lineNumbers: true,
  lineWrapping: false,
  indentWithTabs: true,
  indentUnit: 4,
  tabSize: 4,
  autoCloseBrackets: true,
  matchBrackets: true,
  styleActiveLine: false,
  gutters: ["CodeMirror-linenumbers"],
};

interface Props {
  cfg?: EditorConfiguration;
  className?: string;
  noShadow?: boolean;
  onFocus?: () => void;
  doc?: CodeMirror.Doc;
  sans?: boolean;
}

export default class CM extends React.Component<Props> {
  container: React.RefObject<HTMLDivElement>;
  target: React.RefObject<HTMLDivElement>;
  cm?: CodeMirror.Editor;

  constructor(props: Props) {
    super(props);
    this.container = React.createRef();
    this.target = React.createRef();
  }

  componentDidMount() {
    this.cm = CodeMirror(this.target.current!, this.props.cfg ?? DefaultCfg);
    this.cm.on("focus", () => this.props.onFocus?.());
    if (this.props.doc) {
      this.cm.swapDoc(this.props.doc);
    }
  }

  shouldComponentUpdate(): boolean {
    return false;
  }

  open(doc: CodeMirror.Doc) {
    this.cm!.swapDoc(doc);
  }

  getValue(): string | undefined {
    return this.cm?.getValue();
  }

  render() {
    const shadow = this.props.noShadow ? "" : "shadow-inner";

    return (
      <div
        ref={this.container}
        className={`relative h-full ${
          this.props.sans ? "font-sans" : "font-mono"
        } subpixel-antialiased ${shadow} ${this.props.className}`}
      >
        <style>
          {styles}
        </style>
        <style>
          {encoreStyles}
        </style>
        <style>
          {ideaStyles}
        </style>
        <style>
          {mdStyles}
        </style>
        <style>
          {showHintStyles}
        </style>
        <style>{`
          .CodeMirror {
            height: 100%;
            font-family: inherit;
            // overflow: scroll;
          }
        `}</style>
        <div className="h-full" ref={this.target} />
      </div>
    );
  }
}

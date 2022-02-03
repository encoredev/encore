import React from 'react'

import CodeMirror, {EditorConfiguration} from 'codemirror';
import 'codemirror/lib/codemirror.css'
import './codemirror-show-hint.css';
import './codemirror-encore.css';
import 'codemirror/mode/go/go.js';
import 'codemirror/mode/sql/sql.js';
import 'codemirror/mode/javascript/javascript.js';
import 'codemirror/addon/edit/closebrackets.js';
import 'codemirror/addon/edit/matchbrackets.js';
import 'codemirror/addon/selection/active-line.js';

import 'codemirror/addon/hint/show-hint.js';

export interface TextEdit {
  newText: string;
  range: {
    start: {line: number, character: number};
    end: {line: number, character: number};
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
  gutters: ['CodeMirror-linenumbers'],
}

interface Props {
  cfg?: EditorConfiguration
  className?: string;
  noShadow?: boolean;
  onFocus?: () => void;
}

export default class CM extends React.Component<Props> {
  container: React.RefObject<HTMLDivElement>
  target: React.RefObject<HTMLDivElement>
  cm?: CodeMirror.Editor;

  constructor(props: Props) {
    super(props)
    this.container = React.createRef()
    this.target = React.createRef()
  }

  componentDidMount() {
    this.cm = CodeMirror(this.target.current!, this.props.cfg ?? DefaultCfg)
    this.cm.on("focus", () => this.props.onFocus?.())
  }

  shouldComponentUpdate(): boolean {
    return false
  }

  open(doc: CodeMirror.Doc) {
    this.cm!.swapDoc(doc)
  }

  render() {
    const shadow = this.props.noShadow ? "" : "shadow-inner"

    return (
      <div ref={this.container}
          className={`relative h-full font-mono subpixel-antialiased ${shadow} ${this.props.className}`}>
        <style>{`
          .CodeMirror {
            height: 100%;
            font-family: inherit;
          }
        `}</style>
        <div className="h-full" ref={this.target} />
      </div>
    )
  }
}

export default `
  .cm-s-encore {
    --color-bg: #111111;
    --color-green: #b3d77e;
    --color-yellow: #e9e23d;
    --color-purple: #a36c8c;
    --color-red: #f05c48;
    --color-blue: #6d89ff;
    --color-white: #eeeee1;
    --color-selection: #5f87b5;

    color: var(--color-white);
    border-radius: 0.25rem;
  }

  .request-docs .cm-s-encore,
  .response-docs .cm-s-encore {
    --color-bg: transparent;
  }

  .cm-s-encore.CodeMirror {
    background-color: var(--color-bg);
  }

  .cm-s-encore .CodeMirror-gutters {
    background-color: var(--color-bg);
    border-right: 0;
  }

  .cm-s-encore .CodeMirror-linenumber {
    color: #718096;
    padding: 0 5px;
  }
  .cm-s-encore .CodeMirror-guttermarker-subtle {
    color: #586e75;
  }
  .cm-s-encore .CodeMirror-guttermarker {
    color: #ddd;
  }

  .cm-s-encore .CodeMirror-activeline-background {
    background: rgba(255, 255, 255, 0.06);
  }

  .cm-s-encore .CodeMirror-cursor {
    border-left: 1px solid #e2e8f0;
    color: #718096;
  }

  .cm-s-encore div.CodeMirror-selected {
    background: var(--color-selection);
  }
  .cm-s-encore .CodeMirror-line::selection,
  .cm-s-encore .CodeMirror-line > span::selection,
  .cm-s-encore .CodeMirror-line > span > span::selection {
    background: var(--color-selection);
  }
  .cm-s-encore .CodeMirror-line::-moz-selection,
  .cm-s-encore .CodeMirror-line > span::-moz-selection,
  .cm-s-encore .CodeMirror-line > span > span::-moz-selection {
    background: var(--color-selection);
  }

  .cm-s-encore .cm-header {
    color: #586e75;
  }
  .cm-s-encore .cm-quote {
    color: #93a1a1;
  }
  .cm-s-encore .cm-keyword {
    color: var(--color-blue);
  }
  .cm-s-encore .cm-atom {
    color: var(--color-purple);
  }
  .cm-s-encore .cm-number {
    color: var(--color-yellow);
  }
  .cm-s-encore .cm-def {
    color: var(--color-white);
  }
  .cm-s-encore .cm-variable {
    color: var(--color-white);
  }
  .cm-s-encore .cm-variable-2 {
    color: #b58900;
  }
  .cm-s-encore .cm-variable-3,
  .cm-s-solarized .cm-type {
    color: #6c71c4;
  }
  .cm-s-encore .cm-property {
    color: var(--color-purple);
  }
  .cm-s-encore .cm-operator {
    color: var(--color-blue);
  }
  .cm-s-encore .cm-comment {
    color: var(--color-green);
  }
  .cm-s-encore .cm-string {
    color: var(--color-green);
  }
  .cm-s-encore .cm-string-2 {
    color: #b58900;
  }
  .cm-s-encore .cm-meta {
    color: #859900;
  }
  .cm-s-encore .cm-qualifier {
    color: #b58900;
  }
  .cm-s-encore .cm-builtin {
    color: #d33682;
  }
  .cm-s-encore .cm-bracket {
    color: #cb4b16;
  }
  .cm-s-encore .CodeMirror-matchingbracket {
    background: rgba(255, 255, 255, 0.1);
    color: var(--color-red);
  }
  .cm-s-encore .CodeMirror-nonmatchingbracket {
  }
  .cm-s-encore .cm-tag {
    color: #93a1a1;
  }
  .cm-s-encore .cm-attribute {
    color: #2aa198;
  }
  .cm-s-encore .cm-hr {
    color: transparent;
    border-top: 1px solid #586e75;
    display: block;
  }
  .cm-s-encore .cm-link {
    color: #93a1a1;
    cursor: pointer;
  }
  .cm-s-encore .cm-special {
    color: #6c71c4;
  }
  .cm-s-encore .cm-em {
    color: #999;
    text-decoration: underline;
    text-decoration-style: dotted;
  }
  .cm-s-encore .cm-error,
  .cm-s-encore .cm-invalidchar {
    background-image: url(data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAQAAAACCAYAAAB/qH1jAAAABmJLR0QA/wD/AP+gvaeTAAAACXBIWXMAAAsTAAALEwEAmpwYAAAAB3RJTUUH3QUXCToH00Y1UgAAACFJREFUCNdjPMDBUc/AwNDAAAFMTAwMDA0OP34wQgX/AQBYgwYEx4f9lQAAAABJRU5ErkJggg==);
    background-position: bottom left;
    background-repeat: repeat-x;
  }

  .cm-s-encore .CodeMirror-gutter.webedit {
    width: 10px;
    height: 24px;
  }

  .cm-s-encore .webedit-guttermarker {
    width: 10px;
    height: 10px;
    margin-top: 7px;
    border-radius: 50%;
    background-color: #900;
  }

  /* TODO Customize */
  .cm-s-solarized .cm-header {
    color: #586e75;
  }
  .cm-s-solarized .cm-quote {
    color: #93a1a1;
  }
  .cm-s-solarized .cm-keyword {
    color: #cb4b16;
  }
  .cm-s-solarized .cm-atom {
    color: #d33682;
  }
  .cm-s-solarized .cm-number {
    color: #d33682;
  }
  .cm-s-solarized .cm-def {
    color: #2aa198;
  }
  .cm-s-solarized .cm-variable {
    color: #839496;
  }
  .cm-s-solarized .cm-variable-2 {
    color: #b58900;
  }
  .cm-s-solarized .cm-variable-3,
  .cm-s-solarized .cm-type {
    color: #6c71c4;
  }
  .cm-s-solarized .cm-property {
    color: #2aa198;
  }
  .cm-s-solarized .cm-operator {
    color: #6c71c4;
  }
  .cm-s-solarized .cm-comment {
    color: #586e75;
    font-style: italic;
  }
  .cm-s-solarized .cm-string {
    color: #859900;
  }
  .cm-s-solarized .cm-string-2 {
    color: #b58900;
  }
  .cm-s-solarized .cm-meta {
    color: #859900;
  }
  .cm-s-solarized .cm-qualifier {
    color: #b58900;
  }
  .cm-s-solarized .cm-builtin {
    color: #d33682;
  }
  .cm-s-solarized .cm-bracket {
    color: #cb4b16;
  }
  .cm-s-solarized .CodeMirror-matchingbracket {
    color: #859900;
  }
  .cm-s-solarized .CodeMirror-nonmatchingbracket {
    color: #dc322f;
  }
  .cm-s-solarized .cm-tag {
    color: #93a1a1;
  }
  .cm-s-solarized .cm-attribute {
    color: #2aa198;
  }
  .cm-s-solarized .cm-hr {
    color: transparent;
    border-top: 1px solid #586e75;
    display: block;
  }
  .cm-s-solarized .cm-link {
    color: #93a1a1;
    cursor: pointer;
  }
  .cm-s-solarized .cm-special {
    color: #6c71c4;
  }
  .cm-s-solarized .cm-em {
    color: #999;
    text-decoration: underline;
    text-decoration-style: dotted;
  }
  .cm-s-solarized .cm-error,
  .cm-s-solarized .cm-invalidchar {
    color: #586e75;
    border-bottom: 1px dotted #dc322f;
  }

  /* Gutter colors and line number styling based of color scheme (dark / light) */

  /* Dark */
  .cm-s-solarized.cm-s-dark .CodeMirror-gutters {
    background-color: #073642;
  }

  .cm-s-solarized.cm-s-dark .CodeMirror-linenumber {
    color: #586e75;
    text-shadow: #021014 0 -1px;
  }
`;

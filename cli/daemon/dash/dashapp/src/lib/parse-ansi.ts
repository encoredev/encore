export interface Chunk {
  type: "text" | "ansi" | "newline";
  value: string;
  style: Style;
}

export interface Style {
  foregroundColor?: string;
  backgroundColor?: string;
  dim?: boolean;
  bold?: boolean;
  italic?: boolean;
  underline?: boolean;
  strikethrough?: boolean;
  inverse?: boolean;
}

const ansiRegex = ({onlyFirst = false} = {}) => {
	const pattern = [
		'[\\u001B\\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[-a-zA-Z\\d\\/#&.:=?%@~_]*)*)?\\u0007)',
		'(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PR-TZcf-ntqry=><~]))'
	].join('|');

	return new RegExp(pattern, onlyFirst ? undefined : 'g');
};

const stripAnsi = (string: any) => typeof string === 'string' ? string.replace(ansiRegex(), '') : string;


const ansiTags: {[key: string]: string} = {
  '\u001B[30m': 'black',
  '\u001B[31m': 'red',
  '\u001B[32m': 'green',
  '\u001B[33m': 'yellow',
  '\u001B[34m': 'blue',
  '\u001B[35m': 'magenta',
  '\u001B[36m': 'cyan',
  '\u001B[37m': 'white',

  '\u001B[90m': 'gray',
  '\u001B[91m': 'redBright',
  '\u001B[92m': 'greenBright',
  '\u001B[93m': 'yellowBright',
  '\u001B[94m': 'blueBright',
  '\u001B[95m': 'magentaBright',
  '\u001B[96m': 'cyanBright',
  '\u001B[97m': 'whiteBright',

  '\u001B[39m': 'foregroundColorClose',

  '\u001B[40m': 'bgBlack',
  '\u001B[41m': 'bgRed',
  '\u001B[42m': 'bgGreen',
  '\u001B[43m': 'bgYellow',
  '\u001B[44m': 'bgBlue',
  '\u001B[45m': 'bgMagenta',
  '\u001B[46m': 'bgCyan',
  '\u001B[47m': 'bgWhite',

  '\u001B[100m': 'bgGray',
  '\u001B[101m': 'bgRedBright',
  '\u001B[102m': 'bgGreenBright',
  '\u001B[103m': 'bgYellowBright',
  '\u001B[104m': 'bgBlueBright',
  '\u001B[105m': 'bgMagentaBright',
  '\u001B[106m': 'bgCyanBright',
  '\u001B[107m': 'bgWhiteBright',

  '\u001B[49m': 'backgroundColorClose',

  '\u001B[1m': 'boldOpen',
  '\u001B[2m': 'dimOpen',
  '\u001B[3m': 'italicOpen',
  '\u001B[4m': 'underlineOpen',
  '\u001B[7m': 'inverseOpen',
  '\u001B[8m': 'hiddenOpen',
  '\u001B[9m': 'strikethroughOpen',

  '\u001B[22m': 'boldDimClose',
  '\u001B[23m': 'italicClose',
  '\u001B[24m': 'underlineClose',
  '\u001B[27m': 'inverseClose',
  '\u001B[28m': 'hiddenClose',
  '\u001B[29m': 'strikethroughClose',

  '\u001B[0m': 'reset'
}

const decorators: {[key: string]: string} = {
  black: 'foregroundColorOpen',
  red: 'foregroundColorOpen',
  green: 'foregroundColorOpen',
  yellow: 'foregroundColorOpen',
  blue: 'foregroundColorOpen',
  magenta: 'foregroundColorOpen',
  cyan: 'foregroundColorOpen',
  white: 'foregroundColorOpen',

  gray: 'foregroundColorOpen',
  redBright: 'foregroundColorOpen',
  greenBright: 'foregroundColorOpen',
  yellowBright: 'foregroundColorOpen',
  blueBright: 'foregroundColorOpen',
  magentaBright: 'foregroundColorOpen',
  cyanBright: 'foregroundColorOpen',
  whiteBright: 'foregroundColorOpen',

  bgBlack: 'backgroundColorOpen',
  bgRed: 'backgroundColorOpen',
  bgGreen: 'backgroundColorOpen',
  bgYellow: 'backgroundColorOpen',
  bgBlue: 'backgroundColorOpen',
  bgMagenta: 'backgroundColorOpen',
  bgCyan: 'backgroundColorOpen',
  bgWhite: 'backgroundColorOpen',

  bgGray: 'backgroundColorOpen',
  bgRedBright: 'backgroundColorOpen',
  bgGreenBright: 'backgroundColorOpen',
  bgYellowBright: 'backgroundColorOpen',
  bgBlueBright: 'backgroundColorOpen',
  bgMagentaBright: 'backgroundColorOpen',
  bgCyanBright: 'backgroundColorOpen',
  bgWhiteBright: 'backgroundColorOpen',

  foregroundColorClose: 'foregroundColorClose',
  backgroundColorClose: 'backgroundColorClose',

  boldOpen: 'boldOpen',
  dimOpen: 'dimOpen',
  italicOpen: 'italicOpen',
  underlineOpen: 'underlineOpen',
  inverseOpen: 'inverseOpen',
  hiddenOpen: 'hiddenOpen',
  strikethroughOpen: 'strikethroughOpen',

  boldDimClose: 'boldDimClose',
  italicClose: 'italicClose',
  underlineClose: 'underlineClose',
  inverseClose: 'inverseClose',
  hiddenClose: 'hiddenClose',
  strikethroughClose: 'strikethroughClose',

  reset: 'reset'
}

const arrayUniq = (array: any[]) => [...new Set(array)];

// Atomize
// Splits text into "words" by sticky delimiters [ANSI Escape Seq, \n]
// Eg: words = ['\u001b[37m', 'Line 1', '\n', 'Line 2', '\u001b[39m']
const atomize = (text: string) => {
	const ansies = arrayUniq(text.match(ansiRegex()) as string[])
	const words = superSplit(text, ansies.concat(['\n']))
	return {ansies, words}
}

const parse = (ansi: string) => {
	const plainText = stripAnsi(ansi)

	const result: any = {
		raw: ansi,
		plainText,
		chunks: []
	}

	const {
		ansies,
		words
	} = atomize(ansi)

	const styleStack: any = {
		foregroundColor: [],
		backgroundColor: [],
		boldDim: []
	}

	const getForegroundColor = () => {
		if (styleStack.foregroundColor.length > 0) {
			return styleStack.foregroundColor[styleStack.foregroundColor.length - 1]
		}
		return false
	}

	const getBackgroundColor = () => {
		if (styleStack.backgroundColor.length > 0) {
			return styleStack.backgroundColor[styleStack.backgroundColor.length - 1]
		}
		return false
	}

	const getDim = () => {
		return styleStack.boldDim.includes('dim')
	}

	const getBold = () => {
		return styleStack.boldDim.includes('bold')
	}

	const styleState = {
		italic: false,
		underline: false,
		inverse: false,
		hidden: false,
		strikethrough: false
	}

	let x = 0
	let y = 0
	let nAnsi = 0
	let nPlain = 0

	const bundle = (type: any, value: any) => {
		const chunk: Chunk = {
			type,
			value,
      style: {},
		}

		if (type === 'text' || type === 'ansi') {
			const style = chunk.style

			const foregroundColor = getForegroundColor()
			const backgroundColor = getBackgroundColor()
			const dim = getDim()
			const bold = getBold()

			if (foregroundColor) {
				style.foregroundColor = foregroundColor
			}

			if (backgroundColor) {
				style.backgroundColor = backgroundColor
			}

			if (dim) {
				style.dim = dim
			}

			if (bold) {
				style.bold = bold
			}

			if (styleState.italic) {
				style.italic = true
			}

			if (styleState.underline) {
				style.underline = true
			}

			if (styleState.inverse) {
				style.inverse = true
			}

			if (styleState.strikethrough) {
				style.strikethrough = true
			}
		}

		return chunk
	}

	words.forEach((word: string) => {
		// Newline character
		if (word === '\n') {
			const chunk = bundle('newline', '\n')
			result.chunks.push(chunk)

			x = 0
			y += 1
			nAnsi += 1
			nPlain += 1
			return
		}

		// Text characters
		if (ansies.includes(word) === false) {
			const chunk = bundle('text', word)
			result.chunks.push(chunk)

			x += word.length
			nAnsi += word.length
			nPlain += word.length
			return
		}

		// ANSI Escape characters
		const ansiTag = ansiTags[word]
		const decorator = decorators[ansiTag]
		const color = ansiTag

		if (decorator === 'foregroundColorOpen') {
			styleStack.foregroundColor.push(color)
		}

		if (decorator === 'foregroundColorClose') {
			styleStack.foregroundColor.pop()
		}

		if (decorator === 'backgroundColorOpen') {
			styleStack.backgroundColor.push(color)
		}

		if (decorator === 'backgroundColorClose') {
			styleStack.backgroundColor.pop()
		}

		if (decorator === 'boldOpen') {
			styleStack.boldDim.push('bold')
		}

		if (decorator === 'dimOpen') {
			styleStack.boldDim.push('dim')
		}

		if (decorator === 'boldDimClose') {
			styleStack.boldDim.pop()
		}

		if (decorator === 'italicOpen') {
			styleState.italic = true
		}

		if (decorator === 'italicClose') {
			styleState.italic = false
		}

		if (decorator === 'underlineOpen') {
			styleState.underline = true
		}

		if (decorator === 'underlineClose') {
			styleState.underline = false
		}

		if (decorator === 'inverseOpen') {
			styleState.inverse = true
		}

		if (decorator === 'inverseClose') {
			styleState.inverse = false
		}

		if (decorator === 'strikethroughOpen') {
			styleState.strikethrough = true
		}

		if (decorator === 'strikethroughClose') {
			styleState.strikethrough = false
		}

		if (decorator === 'reset') {
			styleState.strikethrough = false
			styleState.inverse = false
			styleState.italic = false
			styleStack.boldDim = []
			styleStack.backgroundColor = []
			styleStack.foregroundColor = []
		}

		const chunk = bundle('ansi', {
			tag: ansiTag,
			ansi: word,
			decorator
		})

		result.chunks.push(chunk)
		nAnsi += word.length
	})

	return result
}

function splitString(str: any, delimiter: any): any {
	const result: any = []

	str.split(delimiter).forEach((str: any) => {
		result.push(str)
		result.push(delimiter)
	})

	result.pop()

	return result
}

const splitArray = (ary: any, delimiter: any) => {
	let result: any = []

	ary.forEach((part: any) => {
		let subRes: any = []

		part.split(delimiter).forEach((str: any) => {
			subRes.push(str)
			subRes.push(delimiter)
		})

		subRes.pop()
		subRes = subRes.filter((str: any) => {
			if (str) {
				return str
			}
			return undefined
		})

		result = result.concat(subRes)
	})

	return result
}

function superSplit(splittable: any, delimiters: any): any {
	if (delimiters.length === 0) {
		return splittable
	}

	if (typeof splittable === 'string') {
		const delimiter = delimiters[delimiters.length - 1]
		const split = splitString(splittable, delimiter)
		return superSplit(split, delimiters.slice(0, -1))
	}

	if (Array.isArray(splittable)) {
		const delimiter = delimiters[delimiters.length - 1]
		const split = splitArray(splittable, delimiter)
		return superSplit(split, delimiters.slice(0, -1))
	}

	return false
}

export default parse
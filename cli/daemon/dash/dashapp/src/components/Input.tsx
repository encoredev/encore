import React, {FunctionComponent, useEffect, useRef, useState} from 'react'

type Type = "text" | "number" | "email" | "password";

export interface InputProps {
    id: string;
    value: string;
    type?: Type;
    onChange?: (value: string) => void;

    required?: boolean;
    label?: string;
    desc?: string;
    htmlDesc?: string;
    placeholder?: string;
    error?: string;
    prefix?: string;
    cls?: string;
    disabled?: boolean;
}


const Input: FunctionComponent<InputProps> = (props: InputProps) => {
  const typ = props.type || "text"
  const onChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    if (props.onChange) {
      props.onChange(event.target.value)
    }
  }

  const extraCls = props.disabled ? "bg-gray-100 text-gray-600" : ""

  return (
    <div className={props.cls}>
      {props.label &&
        <label htmlFor={props.id} className="block text-sm font-medium leading-5 text-gray-700 mb-1">{props.label}</label>
      }

      {props.error ? (
        <>
          {props.prefix ? (
            <div className="flex relative rounded-md shadow-sm">
              <span className="inline-flex items-center px-3 rounded-l-md border border-r-0 border-red-300 bg-gray-50 text-gray-500 sm:text-sm">
                {props.prefix}
              </span>
              <input id={props.id} type={typ} className={`${extraCls} form-input flex-1 block w-full pl-3 pr-10 py-2 rounded-none rounded-r-md border-red-300 text-red-900 placeholder-red-300 focus:border-red-300 focus:shadow-outline-red sm:text-sm sm:leading-5`}
                  onChange={onChange} disabled={props.disabled} required={props.required} placeholder={props.placeholder} value={props.value} aria-invalid="true" aria-describedby={props.id+"-error"} />
              <div className="absolute inset-y-0 right-0 pr-3 flex items-center pointer-events-none">
                <svg className="h-5 w-5 text-red-500" fill="currentColor" viewBox="0 0 20 20">
                  <path fillRule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z" clipRule="evenodd" />
                </svg>
              </div>
            </div>
          ) : (
            <div className="relative rounded-md shadow-sm">
              <input id={props.id} type={typ} className={`${extraCls} form-input block w-full pr-10 border-red-300 text-red-900 placeholder-red-300 focus:border-red-300 focus:shadow-outline-red sm:text-sm sm:leading-5`}
                  onChange={onChange} disabled={props.disabled} required={props.required} placeholder={props.placeholder} value={props.value} aria-invalid="true" aria-describedby={props.id+"-error"} />
              <div className="absolute inset-y-0 right-0 pr-3 flex items-center pointer-events-none">
                <svg className="h-5 w-5 text-red-500" fill="currentColor" viewBox="0 0 20 20">
                  <path fillRule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z" clipRule="evenodd" />
                </svg>
              </div>
            </div>
          )}
          <p className="mt-2 text-sm text-red-600" id={props.id+"-error"}>{props.error}</p>
        </>
      ) : (
        <>
          {props.prefix ? (
            <div className="flex rounded-md shadow-sm">
              <span className="inline-flex items-center px-3 rounded-l-md border border-r-0 border-gray-300 bg-gray-50 text-gray-500 sm:text-sm">
                {props.prefix}
              </span>
              <input id={props.id} type={typ} className={`${extraCls} form-input flex-1 block w-full px-3 py-2 rounded-none rounded-r-md sm:text-sm sm:leading-5`}
                  onChange={onChange} disabled={props.disabled} required={props.required} placeholder={props.placeholder} value={props.value} aria-describedby={props.desc ? `${props.id}-description` : ""} />
            </div>
          ) : (
            <div className="relative rounded-md shadow-sm">
              <input id={props.id} type={typ} className={`${extraCls} form-input block w-full sm:text-sm sm:leading-5`}
                  onChange={onChange} disabled={props.disabled} required={props.required} placeholder={props.placeholder} value={props.value} aria-describedby={props.desc ? `${props.id}-description` : ""} />
            </div>
          )}

          {props.desc ? (
            <p className="mt-2 text-sm text-gray-500" id={props.id + "-description"}>{props.desc}</p>
          ) : props.htmlDesc ? (
            <p className="mt-2 text-sm text-gray-500" id={props.id + "-description"}
                dangerouslySetInnerHTML={{ __html: props.htmlDesc}} />
          ) : null}
        </>
      )}
    </div>
  )
}

export interface TextAreaProps {
    id: string;
    value: string;
    onChange?: (value: string) => void;

    rows?: number;
    required?: boolean;
    label?: string;
    desc?: string;
    htmlDesc?: string;
    placeholder?: string;
    error?: string;
    cls?: string;
    disabled?: boolean;
}


export const TextArea: FunctionComponent<TextAreaProps> = (props) => {
  const onChange = (event: React.ChangeEvent<HTMLTextAreaElement>) => {
    if (props.onChange) {
      props.onChange(event.target.value)
    }
  }

  const extraCls = props.disabled ? "bg-gray-100 text-gray-600" : ""

  return (
    <div className={props.cls}>
      {props.label &&
        <label htmlFor={props.id} className="block text-sm font-medium leading-5 text-gray-700 mb-1">{props.label}</label>
      }

      {props.error ? (
        <>
          <div className="relative rounded-md shadow-sm">
            <textarea id={props.id} rows={props.rows} className={`${extraCls} form-textarea block w-full border-red-300 text-red-900 placeholder-red-300 focus:border-red-300 focus:shadow-outline-red sm:text-sm sm:leading-5`}
                onChange={onChange} disabled={props.disabled} required={props.required} placeholder={props.placeholder} value={props.value} aria-invalid="true" aria-describedby={props.id+"-error"} />
          </div>
          <p className="mt-2 text-sm text-red-600" id={props.id+"-error"}>{props.error}</p>
        </>
      ) : (
        <>
          <div className="relative rounded-md shadow-sm">
            <textarea id={props.id} rows={props.rows} className={`${extraCls} form-textarea block w-full sm:text-sm sm:leading-5`}
                onChange={onChange} disabled={props.disabled} required={props.required} placeholder={props.placeholder} value={props.value} aria-describedby={props.desc ? `${props.id}-description` : ""} />
          </div>

          {props.desc ? (
            <p className="mt-2 text-sm text-gray-500" id={props.id + "-description"}>{props.desc}</p>
          ) : props.htmlDesc ? (
            <p className="mt-2 text-sm text-gray-500" id={props.id + "-description"}
                dangerouslySetInnerHTML={{ __html: props.htmlDesc}} />
          ) : null}
        </>
      )}
    </div>
  )
}

interface RangeProps {
  id?: string;
  value: number;
  min: number;
  max: number;
  onChange: (value: number, frac: number) => void;

  minLabel?: string;
  maxLabel?: string;
  valueLabel?: string;
  title?: string;
}

export const Range: FunctionComponent<RangeProps> = (props) => {
  const filled = (((props.value - props.min) / (props.max - props.min)) * 100) + "%"
  const slider = useRef<HTMLDivElement>(null)
  const [dragging, setDragging] = useState(false)

  const update = (event: {pageX: number}) => {
    const rect = slider.current?.getBoundingClientRect()
    if (rect) {
      let frac = (event.pageX - rect.left) / rect.width
      frac = Math.max(Math.min(frac, 1), 0)
      const newValue = props.min + Math.round(frac * (props.max - props.min))
      props.onChange(newValue, frac)
    }
  }

  const onMouseUp = (event: MouseEvent) => {
    if (event.button !== 0 || !dragging) {
      return
    }

    const rect = slider.current?.getBoundingClientRect()
    const x = event.pageX
    const y = event.pageY
    if (rect && rect.left <= x && x <= rect.right && rect.top <= y && y <= rect.bottom) {
      update(event)
    }

    setDragging(false)
    event.stopPropagation()
    event.preventDefault()
  }

  const onMouseDown = (event: React.MouseEvent) => {
    if (event.button !== 0) {
      return
    }
    setDragging(true)
    event.stopPropagation()
    event.preventDefault()
  }

  const onClick = (event: React.MouseEvent) => {
    if (event.button !== 0) {
      return
    }
    update(event)
    event.stopPropagation()
    event.preventDefault()
  }

  useEffect(() => {
    document.addEventListener("mouseup", onMouseUp)
    return () => document.removeEventListener("mouseup", onMouseUp)
  })

  const onMouseMove = (event: React.MouseEvent) => {
    if (dragging && props.onChange) {
      const rect = slider.current?.getBoundingClientRect()
      if (rect) {
        let frac = (event.pageX - rect.left) / rect.width
        frac = Math.max(Math.min(frac, 1), 0)
        const newValue = props.min + Math.round(frac * (props.max - props.min))
        props.onChange(newValue, frac)
      }
    }
    event.stopPropagation()
    event.preventDefault()
  }

  return (
    <div className="flex justify-center h-8">
      <div className="pt-1 pb-6 relative min-w-full">
        <input type="hidden" id={props.id} value={props.value} />
        <div className="group h-2 bg-gray-200 rounded-full cursor-pointer" ref={slider}
          onMouseDown={onMouseDown} onMouseMove={onMouseMove} onClick={onClick}>
          <div className="absolute h-2 rounded-full bg-teal-600 w-0" style={{ width: filled }}></div>

          <div className="absolute h-4 flex items-center justify-center w-4 rounded-full bg-white shadow border border-gray-300 -ml-2 top-0 cursor-pointer select-none" style={{left: filled}}>
            <div className="relative -mt-2 w-1">
              <div className="invisible group-hover:visible absolute z-40 opacity-100 bottom-100 mb-2 left-0 min-w-full" style={{ marginLeft: "-20.5px" }}>
                <div className="relative shadow-md">
                  <div className="bg-black -mt-8 text-white truncate text-xs rounded py-1 px-4">{props.valueLabel ?? props.value}</div>
                  <svg className="absolute text-black w-full h-2 left-0 top-100" x="0px" y="0px" viewBox="0 0 255 255" xmlSpace="preserve">
                    <polygon className="fill-current" points="0,0 127.5,127.5 255,0"></polygon>
                  </svg>
                </div>
              </div>
            </div>
          </div>
          <div className="absolute text-gray-400 -ml-1 bottom-0 left-0 -mb-1">{props.minLabel ?? props.min}</div>
          <div className="absolute text-gray-800 -ml-1 text-center inset-x-0 bottom-0 -mb-1">{props.valueLabel ?? props.value}</div>
          <div className="absolute text-gray-400 -mr-1 bottom-0 right-0 -mb-1">{props.maxLabel ?? props.max}</div>
        </div>
      </div>
    </div>
  )
}

interface CounterProps {
  id?: string;
  label?: string;
  min?: number;
  max?: number;
  value: number;
  onChange: (val: number) => void;
  increment?: (val: number) => number;
  decrement?: (val: number) => number;
}

export const Counter: FunctionComponent<CounterProps> = (props) => {
  const inc = props.increment ?? ((val: number) => val+1)
  const dec = props.decrement ?? ((val: number) => val-1)
  const update = (val: number) => {
    if (props.max && val > props.max) {
      val = props.max
    }
    if (props.min && val < props.min) {
      val = props.min
    }
    props.onChange(val)
  }

  return (
    <div>
      <style>{`
        input[type='number']::-webkit-inner-spin-button,
        input[type='number']::-webkit-outer-spin-button {
          -webkit-appearance: none;
          margin: 0;
        }
        input:focus { outline: none !important; }
        button:focus { outline: none !important; }
      `}</style>
      {props.label &&
        <label htmlFor={props.id} className="w-full text-gray-700 text-sm font-medium">{props.label}</label>
      }
      <div className="flex flex-row h-10 w-full rounded-lg relative bg-transparent mt-1">
        <button onClick={() => update(dec(props.value))} className="bg-gray-300 text-gray-600 hover:text-gray-700 hover:bg-gray-400 h-full w-20 rounded-l cursor-pointer outline-none">
          <span className="m-auto text-2xl font-thin">−</span>
        </button>
        <input id={props.id} type="number" className="outline-none focus:outline-none text-center w-full bg-gray-300 font-semibold text-md hover:text-black focus:text-black md:text-basecursor-default flex items-center text-gray-700"
            value={props.value} min={props.min} max={props.max} onChange={(e) => update(parseInt(e.target.value))}/>
        <button onClick={() => update(inc(props.value))} className="bg-gray-300 text-gray-600 hover:text-gray-700 hover:bg-gray-400 h-full w-20 rounded-r cursor-pointer">
          <span className="m-auto text-2xl font-thin">+</span>
        </button>
      </div>
    </div>
  )
}

export default Input

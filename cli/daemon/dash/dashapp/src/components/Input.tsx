import React, { FunctionComponent, useEffect, useRef, useState } from "react";

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
  const typ = props.type || "text";
  const onChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    if (props.onChange) {
      props.onChange(event.target.value);
    }
  };

  const extraCls = props.disabled ? "bg-gray-100 text-gray-600" : "";

  return (
    <div className={props.cls}>
      {props.label && (
        <label
          htmlFor={props.id}
          className="text-gray-700 mb-1 block text-sm font-medium leading-5"
        >
          {props.label}
        </label>
      )}

      {props.error ? (
        <>
          {props.prefix ? (
            <div className="shadow-sm relative flex rounded-md">
              <span className="border-red-300 bg-gray-50 text-gray-500 inline-flex items-center rounded-l-md border border-r-0 px-3 sm:text-sm">
                {props.prefix}
              </span>
              <input
                id={props.id}
                type={typ}
                className={`${extraCls} form-input border-red-300 text-red-900 placeholder-red-300 focus:border-red-300 focus:shadow-outline-red block w-full flex-1 rounded-none rounded-r-md py-2 pl-3 pr-10 sm:text-sm sm:leading-5`}
                onChange={onChange}
                disabled={props.disabled}
                required={props.required}
                placeholder={props.placeholder}
                value={props.value}
                aria-invalid="true"
                aria-describedby={props.id + "-error"}
              />
              <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3">
                <svg
                  className="text-red-500 h-5 w-5"
                  fill="currentColor"
                  viewBox="0 0 20 20"
                >
                  <path
                    fillRule="evenodd"
                    d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z"
                    clipRule="evenodd"
                  />
                </svg>
              </div>
            </div>
          ) : (
            <div className="shadow-sm relative rounded-md">
              <input
                id={props.id}
                type={typ}
                className={`${extraCls} form-input border-red-300 text-red-900 placeholder-red-300 focus:border-red-300 focus:shadow-outline-red block w-full pr-10 sm:text-sm sm:leading-5`}
                onChange={onChange}
                disabled={props.disabled}
                required={props.required}
                placeholder={props.placeholder}
                value={props.value}
                aria-invalid="true"
                aria-describedby={props.id + "-error"}
              />
              <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3">
                <svg
                  className="text-red-500 h-5 w-5"
                  fill="currentColor"
                  viewBox="0 0 20 20"
                >
                  <path
                    fillRule="evenodd"
                    d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z"
                    clipRule="evenodd"
                  />
                </svg>
              </div>
            </div>
          )}
          <p className="text-red-600 mt-2 text-sm" id={props.id + "-error"}>
            {props.error}
          </p>
        </>
      ) : (
        <>
          {props.prefix ? (
            <div className="shadow-sm flex rounded-md">
              <span className="border-gray-300 bg-gray-50 text-gray-500 inline-flex items-center rounded-l-md border border-r-0 px-3 sm:text-sm">
                {props.prefix}
              </span>
              <input
                id={props.id}
                type={typ}
                className={`${extraCls} form-input block w-full flex-1 rounded-none rounded-r-md px-3 py-2 sm:text-sm sm:leading-5`}
                onChange={onChange}
                disabled={props.disabled}
                required={props.required}
                placeholder={props.placeholder}
                value={props.value}
                aria-describedby={props.desc ? `${props.id}-description` : ""}
              />
            </div>
          ) : (
            <div className="shadow-sm relative rounded-md">
              <input
                id={props.id}
                type={typ}
                className={`${extraCls} form-input block w-full sm:text-sm sm:leading-5`}
                onChange={onChange}
                disabled={props.disabled}
                required={props.required}
                placeholder={props.placeholder}
                value={props.value}
                aria-describedby={props.desc ? `${props.id}-description` : ""}
              />
            </div>
          )}

          {props.desc ? (
            <p
              className="text-gray-500 mt-2 text-sm"
              id={props.id + "-description"}
            >
              {props.desc}
            </p>
          ) : props.htmlDesc ? (
            <p
              className="text-gray-500 mt-2 text-sm"
              id={props.id + "-description"}
              dangerouslySetInnerHTML={{ __html: props.htmlDesc }}
            />
          ) : null}
        </>
      )}
    </div>
  );
};

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
      props.onChange(event.target.value);
    }
  };

  const extraCls = props.disabled ? "bg-gray-100 text-gray-600" : "";

  return (
    <div className={props.cls}>
      {props.label && (
        <label
          htmlFor={props.id}
          className="text-gray-700 mb-1 block text-sm font-medium leading-5"
        >
          {props.label}
        </label>
      )}

      {props.error ? (
        <>
          <div className="shadow-sm relative rounded-md">
            <textarea
              id={props.id}
              rows={props.rows}
              className={`${extraCls} form-textarea border-red-300 text-red-900 placeholder-red-300 focus:border-red-300 focus:shadow-outline-red block w-full sm:text-sm sm:leading-5`}
              onChange={onChange}
              disabled={props.disabled}
              required={props.required}
              placeholder={props.placeholder}
              value={props.value}
              aria-invalid="true"
              aria-describedby={props.id + "-error"}
            />
          </div>
          <p className="text-red-600 mt-2 text-sm" id={props.id + "-error"}>
            {props.error}
          </p>
        </>
      ) : (
        <>
          <div className="shadow-sm relative rounded-md">
            <textarea
              id={props.id}
              rows={props.rows}
              className={`${extraCls} form-textarea block w-full sm:text-sm sm:leading-5`}
              onChange={onChange}
              disabled={props.disabled}
              required={props.required}
              placeholder={props.placeholder}
              value={props.value}
              aria-describedby={props.desc ? `${props.id}-description` : ""}
            />
          </div>

          {props.desc ? (
            <p
              className="text-gray-500 mt-2 text-sm"
              id={props.id + "-description"}
            >
              {props.desc}
            </p>
          ) : props.htmlDesc ? (
            <p
              className="text-gray-500 mt-2 text-sm"
              id={props.id + "-description"}
              dangerouslySetInnerHTML={{ __html: props.htmlDesc }}
            />
          ) : null}
        </>
      )}
    </div>
  );
};

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
  const filled =
    ((props.value - props.min) / (props.max - props.min)) * 100 + "%";
  const slider = useRef<HTMLDivElement>(null);
  const [dragging, setDragging] = useState(false);

  const update = (event: { pageX: number }) => {
    const rect = slider.current?.getBoundingClientRect();
    if (rect) {
      let frac = (event.pageX - rect.left) / rect.width;
      frac = Math.max(Math.min(frac, 1), 0);
      const newValue = props.min + Math.round(frac * (props.max - props.min));
      props.onChange(newValue, frac);
    }
  };

  const onMouseUp = (event: MouseEvent) => {
    if (event.button !== 0 || !dragging) {
      return;
    }

    const rect = slider.current?.getBoundingClientRect();
    const x = event.pageX;
    const y = event.pageY;
    if (
      rect &&
      rect.left <= x &&
      x <= rect.right &&
      rect.top <= y &&
      y <= rect.bottom
    ) {
      update(event);
    }

    setDragging(false);
    event.stopPropagation();
    event.preventDefault();
  };

  const onMouseDown = (event: React.MouseEvent) => {
    if (event.button !== 0) {
      return;
    }
    setDragging(true);
    event.stopPropagation();
    event.preventDefault();
  };

  const onClick = (event: React.MouseEvent) => {
    if (event.button !== 0) {
      return;
    }
    update(event);
    event.stopPropagation();
    event.preventDefault();
  };

  useEffect(() => {
    document.addEventListener("mouseup", onMouseUp);
    return () => document.removeEventListener("mouseup", onMouseUp);
  });

  const onMouseMove = (event: React.MouseEvent) => {
    if (dragging && props.onChange) {
      const rect = slider.current?.getBoundingClientRect();
      if (rect) {
        let frac = (event.pageX - rect.left) / rect.width;
        frac = Math.max(Math.min(frac, 1), 0);
        const newValue = props.min + Math.round(frac * (props.max - props.min));
        props.onChange(newValue, frac);
      }
    }
    event.stopPropagation();
    event.preventDefault();
  };

  return (
    <div className="flex h-8 justify-center">
      <div className="relative min-w-full pt-1 pb-6">
        <input type="hidden" id={props.id} value={props.value} />
        <div
          className="bg-gray-200 group h-2 cursor-pointer rounded-full"
          ref={slider}
          onMouseDown={onMouseDown}
          onMouseMove={onMouseMove}
          onClick={onClick}
        >
          <div
            className="bg-teal-600 absolute h-2 w-0 rounded-full"
            style={{ width: filled }}
          ></div>

          <div
            className="shadow border-gray-300 absolute top-0 -ml-2 flex h-4 w-4 cursor-pointer select-none items-center justify-center rounded-full border bg-white"
            style={{ left: filled }}
          >
            <div className="relative -mt-2 w-1">
              <div
                className="bottom-100 invisible absolute left-0 z-40 mb-2 min-w-full opacity-100 group-hover:visible"
                style={{ marginLeft: "-20.5px" }}
              >
                <div className="shadow-md relative">
                  <div className="-mt-8 truncate rounded bg-black py-1 px-4 text-xs text-white">
                    {props.valueLabel ?? props.value}
                  </div>
                  <svg
                    className="top-100 absolute left-0 h-2 w-full text-black"
                    x="0px"
                    y="0px"
                    viewBox="0 0 255 255"
                    xmlSpace="preserve"
                  >
                    <polygon
                      className="fill-current"
                      points="0,0 127.5,127.5 255,0"
                    ></polygon>
                  </svg>
                </div>
              </div>
            </div>
          </div>
          <div className="text-gray-400 absolute bottom-0 left-0 -ml-1 -mb-1">
            {props.minLabel ?? props.min}
          </div>
          <div className="text-gray-800 absolute inset-x-0 bottom-0 -ml-1 -mb-1 text-center">
            {props.valueLabel ?? props.value}
          </div>
          <div className="text-gray-400 absolute bottom-0 right-0 -mr-1 -mb-1">
            {props.maxLabel ?? props.max}
          </div>
        </div>
      </div>
    </div>
  );
};

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
  const inc = props.increment ?? ((val: number) => val + 1);
  const dec = props.decrement ?? ((val: number) => val - 1);
  const update = (val: number) => {
    if (props.max && val > props.max) {
      val = props.max;
    }
    if (props.min && val < props.min) {
      val = props.min;
    }
    props.onChange(val);
  };

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
      {props.label && (
        <label
          htmlFor={props.id}
          className="text-gray-700 w-full text-sm font-medium"
        >
          {props.label}
        </label>
      )}
      <div className="relative mt-1 flex h-10 w-full flex-row rounded-lg bg-transparent">
        <button
          onClick={() => update(dec(props.value))}
          className="bg-gray-300 text-gray-600 hover:text-gray-700 hover:bg-gray-400 outline-none h-full w-20 cursor-pointer rounded-l"
        >
          <span className="m-auto text-2xl font-thin">−</span>
        </button>
        <input
          id={props.id}
          type="number"
          className="outline-none focus:outline-none bg-gray-300 text-md md:text-basecursor-default text-gray-700 flex w-full items-center text-center font-semibold focus:text-black hover:text-black"
          value={props.value}
          min={props.min}
          max={props.max}
          onChange={(e) => update(parseInt(e.target.value))}
        />
        <button
          onClick={() => update(inc(props.value))}
          className="bg-gray-300 text-gray-600 hover:text-gray-700 hover:bg-gray-400 h-full w-20 cursor-pointer rounded-r"
        >
          <span className="m-auto text-2xl font-thin">+</span>
        </button>
      </div>
    </div>
  );
};

export default Input;

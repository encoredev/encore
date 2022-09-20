import React, {
  FC,
  FunctionComponent,
  useEffect,
  useRef,
  useState,
} from "react";
import { Icon } from "./icons";

export type InputType = "text" | "number" | "email" | "password";

export interface InputProps {
  id?: string;
  label?: string;
  value: string;
  type?: InputType;
  placeholder?: string;

  onChange?: (value: string) => void;
  required?: boolean;
  desc?: string | JSX.Element;
  error?: string;
  prefix?: string;
  prefixIcon?: Icon;
  className?: string;
  disabled?: boolean;
  autoComplete?: boolean;
  autoFocus?: boolean;

  noInputWrapper?: boolean;
}

const Input: FunctionComponent<InputProps> = (props: InputProps) => {
  const typ = props.type || "text";

  const onChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    if (props.onChange) {
      props.onChange(event.target.value);
    }
  };

  const autoComplete =
    props.autoComplete !== undefined
      ? props.autoComplete
        ? "on"
        : "off"
      : undefined;

  const input = (
    <input
      id={props.id}
      type={typ}
      className="
              lead-xxsmall h-10 w-full

              border-2 border-b-4

              border-black bg-white

              pl-10px pr-10px pt-8px pb-8px normal-case
              text-black placeholder:font-mono placeholder:text-lead-xxs

              placeholder:font-normal placeholder:uppercase placeholder:text-black placeholder-shown:border-b-2

              placeholder-shown:pb-8px focus:border-black focus:ring-0

              disabled:cursor-not-allowed disabled:border-opacity-50
              disabled:text-opacity-50 disabled:placeholder:text-opacity-50 dark:border-white dark:bg-black dark:text-white

              dark:placeholder:text-white
              dark:focus:border-white dark:disabled:border-opacity-50 dark:disabled:text-opacity-50
              dark:disabled:placeholder:text-opacity-50 mobile:h-10 placeholder:mobile:text-mobile-lead-s
            "
      onChange={onChange}
      disabled={props.disabled}
      required={props.required}
      placeholder={props.placeholder ?? props.label}
      autoComplete={autoComplete}
      value={props.value}
      aria-invalid={props.error ? "true" : "false"}
      aria-describedby={props.id ? props.id + "-error" : undefined}
      autoFocus={props.autoFocus}
    />
  );

  return (
    <div className={props.className}>
      {input}
      {props.desc && (
        <p className="mt-2 text-sm text-black" id={props.id + "-description"}>
          {props.desc}
        </p>
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
  desc?: string | JSX.Element;
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

  const extraCls = props.disabled ? "text-black text-opacity-60" : "";

  return (
    <div className={props.cls}>
      {props.label && (
        <label
          htmlFor={props.id}
          className="mb-1 block text-sm font-medium leading-5 text-black"
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
          <p
            className="mt-2 text-sm text-validation-fail"
            id={props.id + "-error"}
          >
            {props.error}
          </p>
        </>
      ) : (
        <>
          <div className="shadow-sm relative rounded-md">
            <textarea
              id={props.id}
              rows={props.rows}
              className={`${extraCls} form-textarea block w-full border-2 border-black bg-white sm:text-sm sm:leading-5`}
              onChange={onChange}
              disabled={props.disabled}
              required={props.required}
              placeholder={props.placeholder}
              value={props.value}
              aria-describedby={props.desc ? `${props.id}-description` : ""}
            />
          </div>

          {props.desc && (
            <p
              className="mt-2 text-sm text-black"
              id={props.id + "-description"}
            >
              {props.desc}
            </p>
          )}
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
        input[type="number"]::-webkit-inner-spin-button,
        input[type="number"]::-webkit-outer-spin-button {
          -webkit-appearance: none;
          margin: 0;
        }

        input:focus {
          outline: none !important;
        }

        button:focus {
          outline: none !important;
        }
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
          className="bg-coolgray-300 text-gray-600 hover:text-gray-700 hover:bg-coolgray-400 outline-none h-full w-20 cursor-pointer rounded-l"
        >
          <span className="m-auto text-2xl font-thin">−</span>
        </button>
        <input
          id={props.id}
          type="number"
          className="outline-none focus:outline-none bg-coolgray-300 text-md md:text-basecursor-default text-gray-700 flex w-full items-center border-none text-center font-semibold focus:text-black focus:ring-0 hover:text-black"
          value={props.value}
          min={props.min}
          max={props.max}
          onChange={(e) => update(parseInt(e.target.value))}
        />
        <button
          onClick={() => update(inc(props.value))}
          className="bg-coolgray-300 text-gray-600 hover:text-gray-700 hover:bg-coolgray-400 h-full w-20 cursor-pointer rounded-r"
        >
          <span className="m-auto text-2xl font-thin">+</span>
        </button>
      </div>
    </div>
  );
};

export default Input;

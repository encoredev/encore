import React, {FunctionComponent, MouseEventHandler, PropsWithChildren} from "react";

export interface Props extends PropsWithChildren {
  theme: "purple" | "purple:secondary" | "purple:border" | "white" | "red" | "red:secondary" | "gray" | "gray:border";
  size: "xxs" | "xs" | "sm" | "md" | "lg" | "xl";
  cls?: string;
  disabled?: boolean;
  onClick?: MouseEventHandler<HTMLButtonElement>;
  type?: "button" | "submit";
}

const sizeClasses = {
  "xxs": "px-1 py-0.5 text-xs leading-4 rounded",
  "xs": "px-2.5 py-1.5 text-xs leading-4 rounded",
  "sm": "px-3 py-2 text-sm leading-4 rounded-md",
  "md": "px-4 py-2 text-sm leading-5 rounded-md",
  "lg": "px-4 py-2 text-base leading-6 rounded-md",
  "xl": "px-6 py-3 text-base leading-6 rounded-md",
}

const enabledClasses = {
  "purple": "border-transparent text-white bg-purple-600 hover:bg-purple-500 focus:outline-none focus:border-purple-700 focus:shadow-outline-purple active:bg-purple-700",
  "purple:secondary": "border-transparent text-purple-700 bg-purple-100 hover:bg-purple-50 focus:outline-none focus:border-purple-300 focus:shadow-outline-purple active:bg-purple-200",
  "purple:border": "border-purple-600 text-purple-700 bg-white hover:text-purple-500 hover:bg-purple-50 focus:outline-none focus:border-purple-500 focus:shadow-outline-purple active:text-purple-800 active:bg-gray-50",
  "white": "border-gray-300 text-gray-700 bg-white hover:text-gray-500 focus:outline-none focus:border-purple-300 focus:shadow-outline-purple active:text-gray-800 active:bg-gray-50",
  "red": "border-transparent bg-red-600 text-white hover:bg-red-500 focus:outline-none focus:border-red-700 focus:shadow-outline-red active:bg-red-700",
  "red:secondary": "border-red-600 text-red-700 bg-white hover:text-white hover:bg-red-600 focus:outline-none focus:border-red-500 focus:shadow-outline-red active:text-white active:bg-red-600",
  "gray": "border-transparent text-white bg-gray-700 hover:bg-gray-600 focus:outline-none active:bg-gray-800",
  "gray:border": "border-gray-700 text-gray-800 bg-white hover:text-gray-600 hover:bg-gray-50 focus:outline-none focus:border-gray-600 active:text-gray-800 active:bg-gray-50",
}

const disabledClasses = {
  "purple": "border-transparent text-white bg-purple-500 opacity-50 cursor-not-allowed focus:outline-none",
  "purple:secondary": "border-transparent text-purple-700 bg-purple-100 opacity-50 cursor-not-allowed focus:outline-none",
  "purple:border": "border-gray-200 text-gray-600 bg-white opacity-50 cursor-not-allowed focus:outline-none",
  "white": "border-gray-200 text-gray-600 bg-white opacity-50 cursor-not-allowed focus:outline-none",
  "red": "border-transparent text-white bg-red-500 opacity-50 cursor-not-allowed focus:outline-none",
  "red:secondary": "border-gray-200 text-red-800 bg-white opacity-50 cursor-not-allowed focus:outline-none",
  "gray": "border-transparent text-white bg-gray-500 opacity-50 cursor-not-allowed focus:outline-none",
  "gray:border": "border-gray-200 text-gray-600 bg-white opacity-50 cursor-not-allowed focus:outline-none",
}

const Button: FunctionComponent<Props> = (props) => {
  const baseCls = "inline-flex justify-center items-center border font-medium transition duration-150 ease-in-out"
  const cls = `${baseCls} ${props.disabled ? disabledClasses[props.theme] : enabledClasses[props.theme]} ${sizeClasses[props.size]} ${props.cls || ""}`
  return <button onClick={props.onClick} type={props.type ?? "button"} className={cls}>{props.children}</button>
}

export default Button

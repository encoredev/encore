import React, { MouseEventHandler } from "react";

export type Props = React.PropsWithChildren<{
  id?: string;
  className?: string;
  buttonClassName?: string;
  kind: "primary" | "secondary" | "danger";
  section?: "white" | "black";
  disabled?: boolean;
  loading?: boolean;
  type?: "button" | "submit";
  title?: string;
  ariaLabel?: string;

  onClick?: MouseEventHandler<HTMLButtonElement>;
  onMouseDown?: MouseEventHandler<HTMLButtonElement>;
  onMouseUp?: MouseEventHandler<HTMLButtonElement>;
}>;

const classes = {
  primary: {
    white: "bg-black dark:bg-white text-white dark:text-black",
    black: "bg-white text-black",
  },
  secondary: {
    white:
      "bg-white dark:bg-black text-black dark:text-white border-2 border-black dark:border-white",
    black: "bg-black text-white border-2 border-white",
  },
  danger: {
    white: "bg-black dark:bg-white text-white dark:text-black",
    black: "bg-white text-black",
  },
};

const hoverClasses = {
  primary: {
    white: "bg-gradient-to-r brandient-5",
    black: "bg-gradient-to-r brandient-5",
  },
  secondary: { white: "bg-black dark:bg-white", black: "bg-white" },
  danger: {
    white: "bg-[url('/assets/img/fire.gif')] bg-bottom bg-cover",
    black: "bg-red",
  },
};

export type Ref = HTMLButtonElement;

const Button = React.forwardRef<Ref, Props>((props, ref) => {
  const disabled = props.disabled || props.loading;

  const pos =
    (props.className ?? "").indexOf("absolute") === -1 ? "relative" : "";
  const section = props.section ?? "white";

  return (
    <div
      className={`group relative inline-block h-10 mobile:h-10 ${
        props.className ?? ""
      }`}
    >
      {!disabled && (
        <div
          className={`
          absolute inset-0
          ${
            props.kind
              ? hoverClasses[props.kind][section]
              : hoverClasses["primary"][section]
          }
          ${props.buttonClassName ?? ""}
        `}
        />
      )}
      <button
        id={props.id}
        ref={ref}
        onClick={props.onClick}
        onMouseDown={props.onMouseDown}
        onMouseUp={props.onMouseUp}
        type={props.type ?? "button"}
        title={props.title}
        disabled={disabled}
        aria-label={props.ariaLabel}
        className={`
          lead-xxsmall inline-flex h-full w-full
          items-center justify-center px-6 font-mono uppercase mobile:px-4

          ${
            props.kind
              ? classes[props.kind][section]
              : classes["primary"][section]
          } ${pos}

          transition-transform duration-100 ease-in-out
          disabled:cursor-not-allowed disabled:opacity-50
          group-hover:-translate-x-1 group-hover:-translate-y-1

          disabled:group-hover:translate-x-0
          disabled:group-hover:translate-y-0 group-active:-translate-x-1
          group-active:-translate-y-1 disabled:group-active:translate-x-0
          disabled:group-active:translate-y-0
          ${props.buttonClassName ?? ""}
        `}
      >
        {props.children}
      </button>
    </div>
  );
});

export default Button;

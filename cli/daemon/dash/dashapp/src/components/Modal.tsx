import React, {PropsWithChildren} from "react";
import ReactDOM from "react-dom";
import {Transition} from "@headlessui/react"

interface Props extends PropsWithChildren {
  show: boolean;
  close?: () => void;
  width?: string;
}

export class Modal extends React.Component<Props> {
  el?: HTMLDivElement;
  root?: HTMLElement;
  bgRef: React.RefObject<HTMLDivElement>;

  constructor(props: Props) {
    super(props)
    this.handleClick = this.handleClick.bind(this)
    this.handleKeyPress = this.handleKeyPress.bind(this)
    this.bgRef = React.createRef()
  }

  componentDidMount() {
    this.el = document.createElement("div");
    const root = document.getElementById("modal-root")
    if (root === null) {
      throw new Error("could not find #modal-root element")
    }
    this.root = root
    this.root.appendChild(this.el);
    window.addEventListener("keyup", this.handleKeyPress)
  }

  componentWillUnmount() {
    window.removeEventListener("keypress", this.handleKeyPress)
    if (this.root && this.el) {
      this.root.removeChild(this.el)
    }
  }

  handleClick(ev: React.MouseEvent) {
    ev.stopPropagation()
    if (ev.target === this.bgRef.current && this.props.close) {
      this.props.close()
    }
  }

  handleKeyPress(e: KeyboardEvent) {
    if(e.key === "Escape" && this.props.close) {
      this.props.close()
    }
  }

  render() {
    if (!this.el) {
      return null
    }
    const width = this.props.width ?? "sm:max-w-lg sm:w-full"

    return ReactDOM.createPortal(
      (
        <div className="fixed inset-0 pointer-events-none z-40">
          <Transition
              show={this.props.show}
              appear={true}
              enter="ease-linear duration-300"
              enterFrom="opacity-0"
              enterTo="opacity-100"
              leave="ease-linear duration-300"
              leaveFrom="opacity-100"
              leaveTo="opacity-0"
              className="transition-opacity">
            <div className="fixed inset-0">
              <div className="absolute inset-0 bg-gray-500 opacity-75 pointer-events-auto" onClick={this.handleClick} ref={this.bgRef}></div>
            </div>
          </Transition>

          <Transition
              show={this.props.show}
              appear={true}
              enter="ease-out duration-300"
              enterFrom="opacity-0 translate-y-4 sm:translate-y-0 sm:scale-95"
              enterTo="opacity-100 translate-y-0 sm:scale-100"
              leave="ease-in duration-200"
              leaveFrom="opacity-100 translate-y-0 sm:scale-100"
              leaveTo="opacity-0 translate-y-4 sm:translate-y-0 sm:scale-95"
              className="transition-all transform fixed bottom-0 inset-x-0 px-4 pb-4 sm:inset-0 sm:flex sm:items-center sm:justify-center pointer-events-none">
            <div className={`bg-white rounded-lg px-4 pt-5 pb-4 overflow-hidden shadow-xl ${width} sm:p-6 pointer-events-auto`}>
              {this.props.children}
            </div>
          </Transition>
        </div>
      ),
      this.el
    );
  }
}

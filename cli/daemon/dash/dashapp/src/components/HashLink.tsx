/*
This is a vendored version of https://github.com/rafgraph/react-router-hash-link
with typescript type information added.

The MIT License (MIT)

Copyright (c) 2017 Rafael Pedicini

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

import React from "react";
import { Link, NavLink } from "react-router-dom";

let hashFragment = "";
let observer: MutationObserver | null = null;
let asyncTimerId: number | null = null;
let scrollFunction: ((el: HTMLElement) => void) | null = null;

export interface Props extends React.PropsWithChildren<{ className?: string }> {
  to: string | { [key: string]: any };
  elementId?: string;
  onClick?: (e: React.MouseEvent) => void;
  target?: string;
  timeout?: number;
  scroll?: (el: HTMLElement) => void;
  smooth?: boolean;
}

function reset() {
  hashFragment = "";
  if (observer !== null) observer.disconnect();
  if (asyncTimerId !== null) {
    window.clearTimeout(asyncTimerId);
    asyncTimerId = null;
  }
}

function isInteractiveElement(element: HTMLElement) {
  const formTags = ["BUTTON", "INPUT", "SELECT", "TEXTAREA"];
  const linkTags = ["A", "AREA"];
  return (
    (formTags.includes(element.tagName) && !element.hasAttribute("disabled")) ||
    (linkTags.includes(element.tagName) && element.hasAttribute("href"))
  );
}

function getElAndScroll() {
  let element = null;
  if (hashFragment === "#") {
    // use document.body instead of document.documentElement because of a bug in smoothscroll-polyfill in safari
    // see https://github.com/iamdustan/smoothscroll/issues/138
    // while smoothscroll-polyfill is not included, it is the recommended way to implement smoothscroll
    // in browsers that don't natively support el.scrollIntoView({ behavior: 'smooth' })
    element = document.body;
  } else {
    // check for element with matching id before assume '#top' is the top of the document
    // see https://html.spec.whatwg.org/multipage/browsing-the-web.html#target-element
    const id = hashFragment.replace("#", "");
    element = document.getElementById(id);
    if (element === null && hashFragment === "#top") {
      // see above comment for why document.body instead of document.documentElement
      element = document.body;
    }
  }

  if (element !== null) {
    scrollFunction?.(element);

    // update focus to where the page is scrolled to
    // unfortunately this doesn't work in safari (desktop and iOS) when blur() is called
    let originalTabIndex = element.getAttribute("tabindex");
    if (originalTabIndex === null && !isInteractiveElement(element)) {
      element.setAttribute("tabindex", "-1");
    }
    element.focus({ preventScroll: true });
    if (originalTabIndex === null && !isInteractiveElement(element)) {
      // for some reason calling blur() in safari resets the focus region to where it was previously,
      // if blur() is not called it works in safari, but then are stuck with default focus styles
      // on an element that otherwise might never had focus styles applied, so not an option
      element.blur();
      element.removeAttribute("tabindex");
    }

    reset();
    return true;
  }
  return false;
}

function hashLinkScroll(timeout: number | undefined) {
  // Push onto callback queue so it runs after the DOM is updated
  window.setTimeout(() => {
    if (getElAndScroll() === false) {
      if (observer === null) {
        observer = new MutationObserver(getElAndScroll);
      }
      observer.observe(document, {
        attributes: true,
        childList: true,
        subtree: true,
      });
      // if the element doesn't show up in specified timeout or 10 seconds, stop checking
      asyncTimerId = window.setTimeout(() => {
        reset();
      }, timeout || 10000);
    }
  }, 0);
}

export function genericHashLink(As: any) {
  return React.forwardRef<{}, Props>((props, ref) => {
    let linkHash = "";
    if (typeof props.to === "string" && props.to.includes("#")) {
      linkHash = `#${props.to.split("#").slice(1).join("#")}`;
    } else if (typeof props.to === "object" && typeof props.to.hash === "string") {
      linkHash = props.to.hash;
    }

    const passDownProps: { [key: string]: any } = {};
    if (As === NavLink) {
      passDownProps.isActive = (match: any, location: any) =>
        match && match.isExact && location.hash === linkHash;
    }

    function handleClick(e: React.MouseEvent<HTMLElement>) {
      reset();
      hashFragment = props.elementId ? `#${props.elementId}` : linkHash;
      if (props.onClick) props.onClick(e);
      if (
        hashFragment !== "" &&
        // ignore non-vanilla click events, same as react-router
        // below logic adapted from react-router: https://github.com/ReactTraining/react-router/blob/fc91700e08df8147bd2bb1be19a299cbb14dbcaa/packages/react-router-dom/modules/Link.js#L43-L48
        !e.defaultPrevented && // onClick prevented default
        e.button === 0 && // ignore everything but left clicks
        (!props.target || props.target === "_self") && // let browser handle "target=_blank" etc
        !(e.metaKey || e.altKey || e.ctrlKey || e.shiftKey) // ignore clicks with modifier keys
      ) {
        scrollFunction =
          props.scroll ||
          ((el) =>
            props.smooth ? el.scrollIntoView({ behavior: "smooth" }) : el.scrollIntoView());
        hashLinkScroll(props.timeout);
      }
    }
    const { scroll, smooth, timeout, elementId, className, ...filteredProps } = props;
    return (
      <As
        {...passDownProps}
        {...filteredProps}
        className={className}
        onClick={handleClick}
        ref={ref}
      >
        {props.children}
      </As>
    );
  });
}

export const HashLink = genericHashLink(Link);

export const NavHashLink = genericHashLink(NavLink);

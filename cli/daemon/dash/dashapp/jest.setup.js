import "@testing-library/jest-dom/extend-expect";
import { screen } from "@testing-library/react";

/**
 * Get element based on text content.
 * Example: If called with getByTextContent("Simon") and document contains <span><b>Si</b>mon</span> then it would
 * return the span.
 * @param text - Text string to search for in document
 * @returns {boolean}
 */
global.getByTextContent = (text) => {
  return screen.getByText((content, element) => {
    const hasText = (element) => element?.textContent === text;
    const elementHasText = hasText(element);
    const childrenDontHaveText = Array.from(element?.children || []).every(
      (child) => !hasText(child)
    );
    return elementHasText && childrenDontHaveText;
  });
};

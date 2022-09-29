import React from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import Combobox, { ComboboxOptionsItem } from "~c/Combobox";

const TEST_ITEMS: ComboboxOptionsItem[] = [
  { name: "Simon" },
  { name: "Marcus" },
  { name: "Stefan" },
  { name: "Dom" },
  { name: "André" },
];

const setup = (
  options: {
    label?: string;
    selectedItem?: ComboboxOptionsItem;
  } = {}
) => {
  const onChange = jest.fn();
  render(
    <Combobox
      items={TEST_ITEMS}
      onChange={onChange}
      label={options.label || "Label"}
      selectedItem={options.selectedItem || TEST_ITEMS[0]}
    />
  );
  return { onChange };
};

describe("Combobox", () => {
  it("should show label", () => {
    setup({
      label: "Test label",
    });

    expect(screen.getByText(/Test label/i)).toBeInTheDocument();
  });

  it("should show selected item in input", () => {
    setup({
      selectedItem: { name: "Stefan" },
    });

    const input = screen.getByTestId("combobox-input") as HTMLInputElement;
    expect(input.value).toEqual("Stefan");
  });

  it("should open dropdown when clicking on input", async () => {
    setup();

    const input = screen.getByTestId("combobox-input") as HTMLInputElement;
    await userEvent.click(input);

    expect(screen.getByText(/Dom/i)).toBeInTheDocument();
  });

  it("should select all text when focusing on the input", async () => {
    setup({
      selectedItem: { name: "Stefan" },
    });

    const input = screen.getByTestId("combobox-input") as HTMLInputElement;
    // userEvent.type will automatically click the element before typing which will result in all text getting selected.
    await userEvent.type(input, "{backspace}");

    expect(input.value).toEqual("");
  });

  it("should fuzzy search among items when typing in input", async () => {
    setup({
      selectedItem: { name: "Stefan" },
    });

    const input = screen.getByTestId("combobox-input") as HTMLInputElement;
    await userEvent.type(input, "si");

    expect(getByTextContent("Simon")).toBeInTheDocument();
  });
});

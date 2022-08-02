import React, { FC, FocusEvent, useEffect, useRef, useState } from "react";
import fuzzysort from "fuzzysort";
import { Combobox as HeadlessCombobox } from "@headlessui/react";
import * as icons from "~c/icons";

const classNames = (...classes: [string, string | boolean]) =>
  classes.filter(Boolean).join(" ");

export interface ComboboxOptionsItem {
  name: string;
}

interface Props {
  label: string;
  selectedItem: ComboboxOptionsItem;
  items: ComboboxOptionsItem[];
  onChange: (selected: ComboboxOptionsItem) => void;
}

  const Combobox: FC<Props> = ({ label, selectedItem, onChange, items }) => {
  const wrapperRef = useRef<HTMLDivElement>(null);
  const [query, setQuery] = useState<string>("");

  useEffect(() => {
    const handleClickOutside = (event: any) => {
      if (wrapperRef.current && !wrapperRef.current.contains(event.target)) {
        setQuery("");
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
    };
  }, [wrapperRef]);

  const filteredItems = fuzzysort.go(query, items, {
    key: "name",
    all: true,
    threshold: -200,
  });

  return (
    <HeadlessCombobox
      as="div"
      value={selectedItem}
      onChange={onChange}
      ref={wrapperRef}
    >
      {({ open }) => (
        <HeadlessCombobox.Button as="div" className="flex-col">
          <HeadlessCombobox.Label className="block text-sm font-medium text-gray-700">
            {label}
          </HeadlessCombobox.Label>
          <div className="relative mt-1">
            <div className="flex">
              <HeadlessCombobox.Input
                className="focus:outline-none w-full rounded-md border border-gray-300 bg-white py-2 pl-3 pr-10 shadow-sm focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 sm:text-sm"
                onFocus={(event: FocusEvent<HTMLInputElement>) =>
                  event.target.select()
                }
                onChange={(event) => setQuery(event.target.value)}
                displayValue={(item: ComboboxOptionsItem) => item.name}
              />
              {icons.chevronDown("h-5 w-5 mr-2 mt-2 absolute right-0")}
            </div>

            {open && filteredItems.length > 0 && (
              <HeadlessCombobox.Options
                static
                className="focus:outline-none absolute z-10 mt-1 max-h-60 w-full overflow-auto rounded-md bg-white py-1 text-base shadow-lg ring-1 ring-black ring-opacity-5 sm:text-sm"
              >
                {filteredItems.map((filteredItem) => (
                  <HeadlessCombobox.Option
                    key={filteredItem.obj.name}
                    value={filteredItem.obj}
                    className={({ active }) =>
                      classNames(
                        "relative cursor-default select-none py-2 pl-3 pr-9",
                        active ? "bg-indigo-600 text-white" : "text-gray-900"
                      )
                    }
                  >
                    {({ active, selected }) => (
                      <>
                        <span
                          className={classNames(
                            "block truncate",
                            selected && "font-semibold"
                          )}
                        >
                          {query &&
                            fuzzysort.highlight(filteredItem, (m, i) => (
                              <b key={i}>{m}</b>
                            ))}
                          {!query && filteredItem.obj.name}
                        </span>

                        {selected && (
                          <span
                            className={classNames(
                              "absolute inset-y-0 right-0 flex items-center pr-4",
                              active ? "text-white" : "text-indigo-600"
                            )}
                          >
                            {icons.check("h-4 w-5")}
                          </span>
                        )}
                      </>
                    )}
                  </HeadlessCombobox.Option>
                ))}
              </HeadlessCombobox.Options>
            )}
          </div>
        </HeadlessCombobox.Button>
      )}
    </HeadlessCombobox>
  );
};

export default Combobox;

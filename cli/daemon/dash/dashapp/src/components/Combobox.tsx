import React, { FC, FocusEvent, useEffect, useRef, useState } from "react";
import fuzzysort from "fuzzysort";
import { Combobox as HeadlessCombobox } from "@headlessui/react";
import { icons } from "~c/icons";

const classNames = (...classes: [string, string | boolean]) => classes.filter(Boolean).join(" ");

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
    <HeadlessCombobox as="div" value={selectedItem} onChange={onChange} ref={wrapperRef}>
      {({ open }) => (
        <HeadlessCombobox.Button as="div" className="flex-col">
          <HeadlessCombobox.Label className="text-gray-700 block text-sm">
            {label}
          </HeadlessCombobox.Label>
          <div className="relative mt-1">
            <div className="flex">
              <HeadlessCombobox.Input
                className="border-gray-300 shadow-sm w-full border-2 border-black bg-white py-3 pl-3 pr-10 font-mono focus:border-black focus:outline-none focus:ring-0 focus:ring-black sm:text-sm"
                data-testid="combobox-input"
                onFocus={(event: FocusEvent<HTMLInputElement>) => event.target.select()}
                onChange={(event) => setQuery(event.target.value)}
                displayValue={(item: ComboboxOptionsItem) => item.name}
              />
              {icons.chevronDown("h-3 w-3 mr-3 mt-[18px] absolute right-0")}
            </div>

            {open && filteredItems.length > 0 && (
              <HeadlessCombobox.Options
                static
                className="absolute z-10 max-h-96 w-full overflow-auto border-2 border-t-0 border-black bg-white font-mono ring-1 ring-black ring-opacity-5 scrollbar-none focus:outline-none sm:text-sm"
              >
                {filteredItems.map((filteredItem) => (
                  <HeadlessCombobox.Option
                    key={filteredItem.obj.name}
                    value={filteredItem.obj}
                    className={({ active }) =>
                      classNames(
                        "relative cursor-default select-none py-3 pl-3 pr-9 text-black",
                        active ? "bg-black !text-white" : ""
                      )
                    }
                  >
                    {({ active, selected }) => (
                      <>
                        <span className={classNames("block truncate", selected && "font-semibold")}>
                          {query && fuzzysort.highlight(filteredItem, (m, i) => <b key={i}>{m}</b>)}
                          {!query && filteredItem.obj.name}
                        </span>

                        {selected && (
                          <span
                            className={classNames(
                              "absolute inset-y-0 right-0 flex items-center pr-4",
                              active ? "text-white" : ""
                            )}
                          >
                            {icons.check("h-5 w-5")}
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

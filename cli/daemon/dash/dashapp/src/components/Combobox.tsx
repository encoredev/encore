import React, { FC } from "react";

interface OptionsItem {
  name: string;
}

interface Props {
  label: string;
  selectedValue?: string;
  options: OptionsItem[];
  onChange: (e: React.ChangeEvent<HTMLSelectElement>) => void;
}

const Combobox: FC<Props> = ({ label, selectedValue, onChange, options }) => {
  return (
    <div>
      <label
        htmlFor="endpoint"
        className="block text-sm font-medium text-gray-700"
      >
        {label}
      </label>
      <select
        id="endpoint"
        className="focus:outline-none mt-1 block w-full rounded-md border-gray-300 py-2 pl-3 pr-10 text-base focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
        value={selectedValue}
        onChange={onChange}
      >
        {options.map((a) => (
          <option key={a.name} value={a.name}>
            {a.name}
          </option>
        ))}
      </select>
    </div>
  );
};

export default Combobox;

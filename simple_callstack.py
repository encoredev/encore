#!/usr/bin/env python3

import re
import sys

def parse_trace_log(filename):
    with open(filename, 'r') as f:
        lines = f.readlines()

    stack = []
    output = []
    i = 0

    while i < len(lines):
        line = lines[i].strip()
        # Strip ANSI color codes
        line = re.sub(r'\x1b\[[0-9;]*m', '', line)

        # Look for TRACE lines with enter/return
        if 'TRACE' in line and ('enter' in line or 'return' in line):
            # Extract module and action
            if 'enter' in line:
                match = re.search(r'TRACE.*?([^:]+)::([^:]*?):\s*enter', line)
                if match:
                    module = match.group(1).split('::')[-1]  # Get last part
                    func_info = match.group(2) if match.group(2) else "unknown"

                    indent = "  " * len(stack)
                    output.append(f"{indent}📍 {module}::{func_info}")

                    # Check next line for location
                    if i + 1 < len(lines) and 'at' in lines[i + 1]:
                        location = lines[i + 1].strip()
                        # Strip ANSI codes from location too
                        location = re.sub(r'\x1b\[[0-9;]*m', '', location)
                        location = re.sub(r'.*at\s+', '', location)
                        output.append(f"{indent}   └─ {location}")

                    stack.append((module, func_info))

            elif 'return' in line:
                match = re.search(r'return.*?:\s*(.+)', line)
                return_val = match.group(1) if match else "void"

                # Clean up return value
                if len(return_val) > 80:
                    return_val = return_val[:77] + "..."

                if stack:
                    stack.pop()
                    indent = "  " * len(stack)
                    output.append(f"{indent}   ← {return_val}")

        i += 1

    return output

if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("Usage: python3 simple_callstack.py <trace_log_file>")
        sys.exit(1)

    filename = sys.argv[1]
    formatted = parse_trace_log(filename)

    for line in formatted:
        print(line)

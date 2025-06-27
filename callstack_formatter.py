#!/usr/bin/env python3

import re
import sys

def parse_trace_log(filename):
    with open(filename, 'r') as f:
        lines = f.readlines()
    
    stack = []
    output = []
    
    # Pattern to match trace entries
    enter_pattern = r'(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}Z).*TRACE.*enter.*at\s+([^:]+:\d+).*in\s+([^:]+)::(\w+)'
    return_pattern = r'(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}Z).*TRACE.*return.*:\s*(.*).*at\s+([^:]+:\d+).*in\s+([^:]+)::(\w+)'
    
    for line in lines:
        # Check for enter
        match = re.search(enter_pattern, line)
        if match:
            timestamp, location, module, function = match.groups()
            indent = "  " * len(stack)
            output.append(f"{indent}📍 {module}::{function}")
            output.append(f"{indent}   └─ {location}")
            stack.append((module, function, timestamp))
            continue
            
        # Check for return
        match = re.search(return_pattern, line)
        if match:
            timestamp, return_val, location, module, function = match.groups()
            if stack and stack[-1][0] == module and stack[-1][1] == function:
                stack.pop()
                indent = "  " * len(stack)
                # Clean up return value display
                if len(return_val) > 100:
                    return_val = return_val[:97] + "..."
                output.append(f"{indent}   ← {return_val}")
    
    return output

if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("Usage: python3 callstack_formatter.py <trace_log_file>")
        sys.exit(1)
    
    filename = sys.argv[1]
    formatted = parse_trace_log(filename)
    
    # Print first 100 lines to avoid overwhelming output
    for line in formatted[:100]:
        print(line)
    
    if len(formatted) > 100:
        print(f"\n... ({len(formatted) - 100} more lines)")
#!/usr/bin/env python3

import re
import sys
import json
import html

def parse_trace_log(filename):
    with open(filename, 'r') as f:
        lines = f.readlines()
    
    stack = []
    calls = []
    i = 0
    
    while i < len(lines):
        line = lines[i].strip()
        # Strip ANSI color codes
        line = re.sub(r'\x1b\[[0-9;]*m', '', line)
        
        # Look for TRACE lines with enter/return
        if 'TRACE' in line and ('enter' in line or 'return' in line):
            # Extract timestamp
            timestamp_match = re.search(r'(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}Z)', line)
            timestamp = timestamp_match.group(1) if timestamp_match else ""
            
            if 'enter' in line:
                match = re.search(r'TRACE.*?([^:]+)::([^:]*?):\s*enter', line)
                if match:
                    module = match.group(1).split('::')[-1]
                    func_info = match.group(2) if match.group(2) else "unknown"
                    
                    # Get location from next line
                    location = ""
                    if i + 1 < len(lines) and 'at' in lines[i + 1]:
                        location = lines[i + 1].strip()
                        location = re.sub(r'\x1b\[[0-9;]*m', '', location)
                        location = re.sub(r'.*at\s+', '', location)
                    
                    # Extract function name and arguments from the "in" line
                    func_name = func_info
                    args = ""
                    if i + 2 < len(lines) and 'in' in lines[i + 2]:
                        in_line = lines[i + 2].strip()
                        in_line = re.sub(r'\x1b\[[0-9;]*m', '', in_line)
                        
                        # Extract function name from "in module::function_name with args"
                        func_match = re.search(r'in\s+[^:]+::([^\s]+)(?:\s+with\s+(.+))?', in_line)
                        if func_match:
                            func_name = func_match.group(1)
                            if func_match.group(2):
                                args = func_match.group(2)
                    
                    call_data = {
                        'type': 'enter',
                        'module': module,
                        'function': func_name,
                        'location': location,
                        'arguments': args,
                        'timestamp': timestamp,
                        'depth': len(stack),
                        'children': []
                    }
                    
                    if stack:
                        # Add to parent's children
                        stack[-1]['children'].append(call_data)
                    else:
                        # Top level call
                        calls.append(call_data)
                    
                    stack.append(call_data)
                    
            elif 'return' in line:
                match = re.search(r'return.*?:\s*(.+)', line)
                return_val = match.group(1) if match else "void"
                
                # Try to match the return to the correct function in the stack
                # Look for function name in the return line context
                func_match = None
                if i + 2 < len(lines) and 'in' in lines[i + 2]:
                    in_line = lines[i + 2].strip()
                    in_line = re.sub(r'\x1b\[[0-9;]*m', '', in_line)
                    func_match = re.search(r'in\s+[^:]+::([^\s]+)', in_line)
                
                if stack and func_match:
                    # Find the matching function in the stack (from top down)
                    target_func = func_match.group(1)
                    for stack_idx in range(len(stack) - 1, -1, -1):
                        if stack[stack_idx]['function'] == target_func:
                            stack[stack_idx]['return_value'] = return_val
                            stack[stack_idx]['return_timestamp'] = timestamp
                            # Remove this function and all functions above it from stack
                            stack = stack[:stack_idx]
                            break
                elif stack:
                    # Fallback: assign to top of stack
                    current_call = stack.pop()
                    current_call['return_value'] = return_val
                    current_call['return_timestamp'] = timestamp
        
        i += 1
    
    return calls

def generate_html(calls, output_file):
    html_content = """
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Interactive Callstack Viewer</title>
    <style>
        body {
            font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
            background-color: #1e1e1e;
            color: #d4d4d4;
            margin: 0;
            padding: 20px;
            line-height: 1.4;
        }
        .call-entry {
            margin: 2px 0;
            border-left: 2px solid #333;
            padding-left: 10px;
        }
        .call-header {
            cursor: pointer;
            padding: 4px 8px;
            border-radius: 4px;
            transition: background-color 0.2s;
        }
        .call-header:hover {
            background-color: #2d2d30;
        }
        .function-name {
            color: #4fc1ff;
            font-weight: bold;
        }
        .preview {
            color: #808080;
            font-size: 0.85em;
            margin-left: 10px;
            font-style: italic;
            cursor: pointer;
            opacity: 0.7;
        }
        .preview:hover {
            opacity: 1;
            color: #9cdcfe;
        }
        .args-preview {
            color: #ce9178;
        }
        .return-preview {
            color: #4fc1ff;
        }
        
        /* Tooltip styles */
        .tooltip {
            position: relative;
        }
        .tooltip .tooltiptext {
            visibility: hidden;
            background-color: #2d2d30;
            color: #d4d4d4;
            text-align: left;
            border-radius: 6px;
            padding: 8px 12px;
            position: absolute;
            z-index: 1000;
            bottom: 125%;
            left: 0;
            min-width: 300px;
            max-width: 600px;
            font-size: 0.85em;
            line-height: 1.3;
            border: 1px solid #404040;
            box-shadow: 0 4px 8px rgba(0,0,0,0.3);
            white-space: pre-wrap;
            word-wrap: break-word;
            font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
        }
        .tooltip:hover .tooltiptext {
            visibility: visible;
            opacity: 1;
        }
        .tooltip .tooltiptext::after {
            content: "";
            position: absolute;
            top: 100%;
            left: 20px;
            margin-left: -5px;
            border-width: 5px;
            border-style: solid;
            border-color: #2d2d30 transparent transparent transparent;
        }
        .collapsible {
            display: none;
            margin-left: 20px;
            margin-top: 5px;
        }
        .collapsible.expanded {
            display: block;
        }
        .args-section, .return-section {
            background-color: #1e1e1e;
            border: 1px solid #2d2d30;
            border-radius: 3px;
            margin: 3px 0;
            padding: 4px 8px;
            display: none;
        }
        .args-section.expanded, .return-section.expanded {
            display: block;
        }
        .args-header, .return-header {
            color: #858585;
            font-weight: normal;
            font-size: 0.85em;
            margin-bottom: 3px;
            cursor: pointer;
            opacity: 0.7;
        }
        .args-header:hover, .return-header:hover {
            opacity: 1;
        }
        .args-content, .return-content {
            color: #ce9178;
            white-space: pre-wrap;
            font-size: 0.9em;
            max-height: 200px;
            overflow-y: auto;
            display: none;
        }
        .args-content.expanded, .return-content.expanded {
            display: block;
        }
        .toggle-icon {
            display: inline-block;
            width: 12px;
            font-size: 10px;
            color: #cccccc;
        }
        .children {
            margin-left: 15px;
            border-left: 1px solid #404040;
        }
        .expand-all, .collapse-all, .expand-calls, .expand-data {
            background-color: #0e639c;
            color: white;
            border: none;
            padding: 8px 16px;
            margin: 5px;
            border-radius: 4px;
            cursor: pointer;
        }
        .expand-all:hover, .collapse-all:hover, .expand-calls:hover, .expand-data:hover {
            background-color: #1177bb;
        }
    </style>
</head>
<body>
    <h1>Interactive Callstack Viewer</h1>
    <div>
        <button class="expand-calls" onclick="expandCalls()">Expand Calls</button>
        <button class="expand-all" onclick="expandAll()">Expand All</button>
        <button class="collapse-all" onclick="collapseAll()">Collapse All</button>
    </div>
    <div id="callstack">
""" + generate_call_html(calls) + """
    </div>
    
    <script>
        function toggleCall(element) {
            const content = element.nextElementSibling;
            const icon = element.querySelector('.toggle-icon');
            
            if (content.classList.contains('expanded')) {
                content.classList.remove('expanded');
                icon.textContent = '▶';
            } else {
                content.classList.add('expanded');
                icon.textContent = '▼';
            }
        }
        
        function toggleSection(element) {
            const section = element.closest('.args-section, .return-section');
            const content = element.nextElementSibling;
            const icon = element.querySelector('.toggle-icon');
            
            if (content.classList.contains('expanded')) {
                content.classList.remove('expanded');
                section.classList.remove('expanded');
                icon.textContent = '▶';
            } else {
                content.classList.add('expanded');
                section.classList.add('expanded');
                icon.textContent = '▼';
            }
        }
        
        function expandCalls() {
            // Expand only the call structure, not args/return data
            document.querySelectorAll('.collapsible').forEach(el => {
                el.classList.add('expanded');
            });
            document.querySelectorAll('.call-header .toggle-icon').forEach(el => {
                el.textContent = '▼';
            });
        }
        
        function expandAll() {
            document.querySelectorAll('.collapsible').forEach(el => {
                el.classList.add('expanded');
            });
            document.querySelectorAll('.args-section, .return-section').forEach(el => {
                el.classList.add('expanded');
            });
            document.querySelectorAll('.args-content, .return-content').forEach(el => {
                el.classList.add('expanded');
            });
            document.querySelectorAll('.toggle-icon').forEach(el => {
                el.textContent = '▼';
            });
        }
        
        function collapseAll() {
            document.querySelectorAll('.collapsible').forEach(el => {
                el.classList.remove('expanded');
            });
            document.querySelectorAll('.args-section, .return-section').forEach(el => {
                el.classList.remove('expanded');
            });
            document.querySelectorAll('.args-content, .return-content').forEach(el => {
                el.classList.remove('expanded');
            });
            document.querySelectorAll('.toggle-icon').forEach(el => {
                el.textContent = '▶';
            });
        }
        
        function toggleArgsFromPreview(previewElement) {
            // Find the parent call-entry and expand it if needed
            const callHeader = previewElement.closest('.call-header');
            const collapsible = callHeader.nextElementSibling;
            const callToggleIcon = callHeader.querySelector('.toggle-icon');
            
            // Expand the call if not already expanded
            if (!collapsible.classList.contains('expanded')) {
                collapsible.classList.add('expanded');
                callToggleIcon.textContent = '▼';
            }
            
            // Find and toggle the args section
            const argsSection = collapsible.querySelector('.args-section');
            if (argsSection) {
                const argsHeader = argsSection.querySelector('.args-header');
                const argsContent = argsSection.querySelector('.args-content');
                const argsToggleIcon = argsHeader.querySelector('.toggle-icon');
                
                if (argsSection.classList.contains('expanded')) {
                    // Hide the args section
                    argsSection.classList.remove('expanded');
                    argsContent.classList.remove('expanded');
                    argsToggleIcon.textContent = '▶';
                } else {
                    // Show the args section
                    argsSection.classList.add('expanded');
                    argsContent.classList.add('expanded');
                    argsToggleIcon.textContent = '▼';
                }
            }
        }
        
        function toggleReturnFromPreview(previewElement) {
            // Find the parent call-entry and expand it if needed
            const callHeader = previewElement.closest('.call-header');
            const collapsible = callHeader.nextElementSibling;
            const callToggleIcon = callHeader.querySelector('.toggle-icon');
            
            // Expand the call if not already expanded
            if (!collapsible.classList.contains('expanded')) {
                collapsible.classList.add('expanded');
                callToggleIcon.textContent = '▼';
            }
            
            // Find and toggle the return section
            const returnSection = collapsible.querySelector('.return-section');
            if (returnSection) {
                const returnHeader = returnSection.querySelector('.return-header');
                const returnContent = returnSection.querySelector('.return-content');
                const returnToggleIcon = returnHeader.querySelector('.toggle-icon');
                
                if (returnSection.classList.contains('expanded')) {
                    // Hide the return section
                    returnSection.classList.remove('expanded');
                    returnContent.classList.remove('expanded');
                    returnToggleIcon.textContent = '▶';
                } else {
                    // Show the return section
                    returnSection.classList.add('expanded');
                    returnContent.classList.add('expanded');
                    returnToggleIcon.textContent = '▼';
                }
            }
        }
    </script>
</body>
</html>
"""
    
    with open(output_file, 'w') as f:
        f.write(html_content)

def truncate_preview(text, max_length=50):
    """Truncate text for preview display"""
    if not text:
        return ""
    if len(text) <= max_length:
        return text
    return text[:max_length-3] + "..."

def generate_call_html(calls):
    html_parts = []
    
    for call in calls:
        # Generate tooltips
        function_tooltip = f"File: {call.get('location', 'unknown')}\nTimestamp: {call.get('timestamp', 'unknown')}"
        if call.get('return_timestamp'):
            function_tooltip += f"\nReturn: {call.get('return_timestamp')}"
        
        args_tooltip = html.escape(call.get('arguments', '')) if call.get('arguments') else ''
        return_tooltip = html.escape(call.get('return_value', '')) if call.get('return_value') else ''
        
        # Generate previews with tooltips
        args_preview = ""
        return_preview = ""
        
        if call.get('arguments'):
            args_preview = f'''
            <span class="preview args-preview tooltip" onclick="event.stopPropagation(); toggleArgsFromPreview(this)">
                ({truncate_preview(call["arguments"])})
                <span class="tooltiptext">{args_tooltip}</span>
            </span>'''
        
        if call.get('return_value'):
            return_preview = f'''
            <span class="preview return-preview tooltip" onclick="event.stopPropagation(); toggleReturnFromPreview(this)">
                → {truncate_preview(call["return_value"])}
                <span class="tooltiptext">{return_tooltip}</span>
            </span>'''
        
        html_parts.append(f"""
        <div class="call-entry">
            <div class="call-header" onclick="toggleCall(this)">
                <span class="toggle-icon">▶</span>
                <span class="function-name tooltip">
                    {html.escape(call['module'])}::{html.escape(call['function'])}
                    <span class="tooltiptext">{html.escape(function_tooltip)}</span>
                </span>
                {args_preview}
                {return_preview}
            </div>
            <div class="collapsible">
""")
        
        # Add arguments section if present
        if call.get('arguments'):
            html_parts.append(f"""
                <div class="args-section">
                    <div class="args-header" onclick="toggleSection(this)">
                        <span class="toggle-icon">▶</span>
                        Arguments
                    </div>
                    <div class="args-content">{html.escape(call['arguments'])}</div>
                </div>
""")
        
        # Add return value section if present
        if call.get('return_value'):
            html_parts.append(f"""
                <div class="return-section">
                    <div class="return-header" onclick="toggleSection(this)">
                        <span class="toggle-icon">▶</span>
                        Return Value
                    </div>
                    <div class="return-content">{html.escape(call['return_value'])}</div>
                </div>
""")
        
        # Add children
        if call.get('children'):
            html_parts.append('<div class="children">')
            html_parts.append(generate_call_html(call['children']))
            html_parts.append('</div>')
        
        html_parts.append("""
            </div>
        </div>
""")
    
    return ''.join(html_parts)

if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("Usage: python3 interactive_callstack.py <trace_log_file>")
        sys.exit(1)
    
    filename = sys.argv[1]
    calls = parse_trace_log(filename)
    
    output_file = filename.replace('.log', '_callstack.html')
    generate_html(calls, output_file)
    print(f"Interactive callstack viewer generated: {output_file}")
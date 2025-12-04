#!/bin/bash
set -ex

copyright_header='/*
Copyright © 2025 Jayson Grace <jayson.e.grace@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/'

echo "Starting copyright check..."

update_copyright() {
    local file="$1"
    local temp_file
    temp_file=$(mktemp)
    echo "${copyright_header}" > "${temp_file}"
    echo "" >> "${temp_file}"  # Add an empty line after the header
    sed '/^\/\*/,/^\*\//d' "$file" | sed '/./,$!d' >> "${temp_file}"
    mv "${temp_file}" "${file}"
}

# Get the list of staged .go files
staged_files=$(git diff --cached --name-only --diff-filter=ACM | grep '\.go$' || true)

# Check if there are any staged .go files
if [[ -z "$staged_files" ]]; then
    echo "No .go files staged for commit. Exiting."
    exit 0
fi

for file in $staged_files; do
    echo "Checking file: $file"
    if grep -qF "Copyright © 2025 Jayson Grace" "$file"; then
        echo "Current copyright header is up-to-date in $file"
    else
        echo "Updating copyright header in $file"
        update_copyright "$file"
        echo "Copyright header updated in $file"
    fi
done

echo "Copyright check completed."

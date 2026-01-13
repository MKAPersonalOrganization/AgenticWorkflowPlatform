#!/bin/bash
# analyze_imports.sh

MODULE="school-with-ai"

echo "Package,Dependency"
grep -r "import (" . -A 20 | grep "$MODULE" | sed 's/--//g' | while read -r line; do
    file=$(echo "$line" | cut -d- -f1 | cut -d: -f1)
    dep=$(echo "$line" | grep -o "$MODULE/[^\"]*")
    
    if [ ! -z "$file" ] && [ ! -z "$dep" ]; then
        pkg=$(dirname "$file" | sed 's/^\.\///')
        echo "$pkg,$dep"
    fi
done | sort | uniq

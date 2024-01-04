#!/bin/bash

filename="$1"

# Create or clear the coveragetable.md file
touch build/coveragetable.md

# Append total coverage information to coveragetable.md
echo -n "### Total coverage : " >> build/coveragetable.md
# Filter and assign files to trimmed_content based on conditions:
# - Files ending with ".go" are considered.
# - Files with status "A" or "M" are included.
# - Files with status "RXXX" are included, except for "R100".
go tool cover -func build/coverage.out | grep 'total' | awk '{print $3}' >> build/coveragetable.md

# Extract changed file paths from the provided file
trimmed_content=$(grep -E '\.go$' "$filename" | awk '{ if ($1 ~ /^R/ && $1 !~ /^R100/) print $3; else if ($1 ~ /^[AM]/) print $2 }')

if [[ -z "$trimmed_content" ]]; then
  echo "### No Files Are Changed." >> build/coveragetable.md
else
  echo "List of changed files" >> build/coveragetable.md
  echo -e "\n" >> build/coveragetable.md
  echo "| Path | Function | Coverage |" >> build/coveragetable.md
  echo "|------|----------|----------|" >> build/coveragetable.md

  while IFS= read -r line; do
    echo "$line"
    if [[ $line == *.go ]]; then
      # Extract coverage information for each changed file and append to coveragetable.md
      go tool cover -func=build/coverage.out | grep "$line" | awk 'BEGIN{OFS=" | "} {print "|" $1, $2, $3 "|"}' >> build/coveragetable.md
    fi
  done <<< "$trimmed_content"
fi

# Append the content of coveragetable.md to the GITHUB_STEP_SUMMARY file
cat build/coveragetable.md >> "$GITHUB_STEP_SUMMARY"

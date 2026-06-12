#!/usr/bin/env python3
"""
Strip 'description' fields from CRD OpenAPI schemas to reduce file size.
Large CRDs (>256KB) cause 'metadata.annotations: Too long' errors
when applied with client-side kubectl apply.
"""

import sys
import yaml
import os


def strip_descriptions(obj):
    """Recursively remove 'description' keys from a dict/list."""
    if isinstance(obj, dict):
        obj.pop("description", None)
        for value in obj.values():
            strip_descriptions(value)
    elif isinstance(obj, list):
        for item in obj:
            strip_descriptions(item)


def process_file(filepath):
    with open(filepath, "r") as f:
        docs = list(yaml.safe_load_all(f))

    processed = []
    for doc in docs:
        if doc is None:
            continue
        if doc.get("kind") == "CustomResourceDefinition":
            for version in doc.get("spec", {}).get("versions", []):
                schema = version.get("schema", {}).get("openAPIV3Schema", {})
                strip_descriptions(schema)
        processed.append(doc)

    with open(filepath, "w") as f:
        for i, doc in enumerate(processed):
            if i > 0:
                f.write("---\n")
            yaml.safe_dump(doc, f, default_flow_style=False, width=1000, allow_unicode=True)

    size = os.path.getsize(filepath)
    print(f"  {filepath}: {size / 1024:.1f} KB")
    return size


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python3 strip-crd-descriptions.py <file1.yaml> [file2.yaml ...]")
        sys.exit(1)

    print("Stripping description fields from CRDs...")
    for filepath in sys.argv[1:]:
        process_file(filepath)
    print("Done.")

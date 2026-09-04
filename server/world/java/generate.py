"""Regenerate the pinned Java block conversion data used by this package."""
import gzip
import json
from pathlib import Path
from urllib.request import urlopen

COMMIT = "abfaadb4c45e2968a98fc927d32f7c648962d8ad"
BASE = f"https://raw.githubusercontent.com/PrismarineJS/minecraft-data/{COMMIT}/data/"


def read(path):
    """Read one JSON file at the pinned minecraft-data revision."""
    with urlopen(BASE + path, timeout=60) as response:
        return json.load(response)


def canonical(value):
    """Sort property keys so equivalent state strings share one lookup key."""
    name, _, properties = value.partition("[")
    pairs = sorted(filter(None, properties.removesuffix("]").split(",")))
    return name + "[" + ",".join(pairs) + "]"


states = {canonical(key): value for key, value in read("bedrock/1.26.30/blocksJ2B.json").items()}
defaults = {}
for block in read("pc/1.21.11/blocks.json"):
    index = block["defaultState"] - block["minStateId"]
    properties = {}
    for prop in reversed(block["states"]):
        value = index % prop["num_values"]
        index //= prop["num_values"]
        if "values" in prop:
            value = prop["values"][value]
        elif prop["type"] == "bool":
            value = "true" if value == 0 else "false"
        properties[prop["name"]] = str(value)
    defaults["minecraft:" + block["name"]] = properties
mapping = {"states": states, "defaults": defaults, "legacy": read("pc/common/legacy.json")["blocks"]}
data = json.dumps(mapping, sort_keys=True, separators=(",", ":")).encode()
Path(__file__).with_name("mapping.json.gz").write_bytes(gzip.compress(data, mtime=0))

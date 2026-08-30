"""Make the example agents and the SDK importable when pytest collects this dir.

The examples live beside the ``blacklight`` package rather than inside it, so put
both on the path: this directory (for ``import red_team_agent``) and its parent,
``sdk/python`` (for ``import blacklight``).
"""

import os
import sys

_HERE = os.path.dirname(__file__)
for _p in (_HERE, os.path.dirname(_HERE)):
    if _p not in sys.path:
        sys.path.insert(0, _p)

# The example agents infer their Anthropic model at construction time, which the
# provider refuses to do without a key. The tests replace the model entirely with
# a FunctionModel via ``agent.override(...)``, so this placeholder is never used to
# reach the network — it only satisfies the eager constructor at import.
os.environ.setdefault("ANTHROPIC_API_KEY", "test-placeholder-not-used")

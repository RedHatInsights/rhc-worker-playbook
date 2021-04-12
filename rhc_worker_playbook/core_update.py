import sys
import os
from constants import RPM_EGG
from insights_client import run_phase, sorted_eggs, gpg_validate

INSIGHTS_CORE_GPG_CHECK = os.environ.get("INSIGHTS_CORE_GPG_CHECK", "True")

if INSIGHTS_CORE_GPG_CHECK == "True":
    # import the egg to perform the update
    # only use RPM egg to perform the update; it's a verified working egg
    validated_eggs = [gpg_validate(RPM_EGG)]

    if len(validated_eggs) == 0:
        # cannot gpg verify any eggs
        sys.exit(1)

else:
    validated_eggs = [RPM_EGG]
sys.path = validated_eggs + sys.path

from insights.client import InsightsClient
from insights.client.config import InsightsConfig
from insights.client.phase.v1 import get_phases

config = InsightsConfig(verbose=True)

# use a default config + verbose logging to stdout
#   and from_phase=True to utilize the autoconfig
# Majorly greasy
client = InsightsClient(config, True)
print("insights-core version: %s" % client.version())
update = client.update()

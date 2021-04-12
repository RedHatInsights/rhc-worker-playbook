import sys
from constants import RPM_EGG, STABLE_EGG
from insights_client import run_phase, sorted_eggs, gpg_validate

INSIGHTS_CORE_GPG_CHECK = os.environ.get("INSIGHTS_CORE_GPG_CHECK", True)

if INSIGHTS_CORE_GPG_CHECK:
    # import the egg to perform the update
    validated_eggs = sorted_eggs(
        list(filter(gpg_validate, [STABLE_EGG, RPM_EGG])))

    if len(validated_eggs) == 0:
        # cannot gpg verify any eggs
        sys.exit(1)

else:
    validated_eggs = sorted_eggs([STABLE_EGG, RPM_EGG])

sys.path = validated_eggs + sys.path

from insights.client import InsightsClient
from insights.client.phase.v1 import get_phases

# use config=None to use a default config
#   and from_phase=True to utilize the autoconfig
# Majorly greasy
client = InsightsClient(None, True)
update = client.update()

import sys
from constants import RPM_EGG, STABLE_EGG
from insights_client import run_phase, sorted_eggs, gpg_validate

# import the egg to perform the update
validated_eggs = sorted_eggs(
    list(filter(gpg_validate, [STABLE_EGG, RPM_EGG])))
sys.path = validated_eggs + sys.path

from insights.client import InsightsClient
from insights.client.phase.v1 import get_phases

# use config=None to use a default config
#   and from_phase=True to utilize the autoconfig
# Majorly greasy
client = InsightsClient(None, True)
run_phase({"name": "update"}, client, validated_eggs)

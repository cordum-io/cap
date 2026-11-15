from importlib.metadata import version

import cordum_guard


def test_version_is_the_installed_distribution_version():
    assert cordum_guard.__version__ == version("cordum-guard")

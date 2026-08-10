from pathlib import Path

import pytest
import typer

from spawnpoint import templates
from spawnpoint.templates import (
    Template,
    get_template,
    load_templates,
    merge_template,
    preselect_template_repos,
    save_templates,
    template_detail,
    upsert_template,
)


@pytest.fixture
def store(tmp_path, monkeypatch):
    """Point the template store at a temp dir instead of ~/.spawnpoint."""
    path = tmp_path / "templates.toml"
    monkeypatch.setattr(templates, "SPAWNPOINT_DIR", tmp_path)
    monkeypatch.setattr(templates, "TEMPLATES_PATH", path)
    return path


def test_load_returns_empty_when_no_file(store):
    assert load_templates() == {}


def test_round_trip_preserves_fields(store):
    save_templates({
        "image-gen": Template(
            name="image-gen",
            repos=["chottu", "backend"],
            base="staging",
            description="Image generation stack",
        )
    })

    loaded = load_templates()
    assert list(loaded) == ["image-gen"]
    assert loaded["image-gen"].repos == ["chottu", "backend"]
    assert loaded["image-gen"].base == "staging"
    assert loaded["image-gen"].description == "Image generation stack"


def test_optional_fields_round_trip_as_none(store):
    save_templates({"bare": Template(name="bare", repos=["api"])})

    loaded = load_templates()["bare"]
    assert loaded.base is None
    assert loaded.description is None


def test_names_needing_escaping_round_trip(store):
    tricky = 'we"ird\\name.with dots'
    save_templates({
        tricky: Template(
            name=tricky,
            repos=['nested/re"po', "back\\slash"],
            description='quote " and backslash \\',
        )
    })

    loaded = load_templates()
    assert list(loaded) == [tricky]
    assert loaded[tricky].repos == ['nested/re"po', "back\\slash"]
    assert loaded[tricky].description == 'quote " and backslash \\'


def test_templates_are_written_sorted(store):
    save_templates({
        "zeta": Template(name="zeta", repos=["a"]),
        "alpha": Template(name="alpha", repos=["b"]),
    })

    text = store.read_text()
    assert text.index('[templates."alpha"]') < text.index('[templates."zeta"]')


def test_upsert_reports_whether_it_existed(store):
    _, existed = upsert_template(Template(name="pair", repos=["a"]))
    assert existed is False

    _, existed = upsert_template(Template(name="pair", repos=["a", "b"]))
    assert existed is True
    assert load_templates()["pair"].repos == ["a", "b"]


def test_upsert_leaves_other_templates_alone(store):
    upsert_template(Template(name="one", repos=["a"]))
    upsert_template(Template(name="two", repos=["b"]))

    assert sorted(load_templates()) == ["one", "two"]


def test_merge_keeps_saved_base_and_description(store):
    upsert_template(
        Template(name="t", repos=["a"], base="staging", description="original")
    )

    merged = merge_template("t", ["a", "b"])
    assert merged.repos == ["a", "b"]
    assert merged.base == "staging"
    assert merged.description == "original"


def test_merge_overrides_when_given(store):
    upsert_template(Template(name="t", repos=["a"], base="staging", description="old"))

    merged = merge_template("t", ["a"], base="main", description="new")
    assert merged.base == "main"
    assert merged.description == "new"


def test_get_template_exits_on_unknown_name(store):
    upsert_template(Template(name="known", repos=["a"]))

    with pytest.raises(typer.Exit) as excinfo:
        get_template("unknown")
    assert excinfo.value.exit_code == 1


def test_malformed_toml_exits_cleanly(store):
    store.write_text("this is not = = valid toml\n")

    with pytest.raises(typer.Exit) as excinfo:
        load_templates()
    assert excinfo.value.exit_code == 1


def test_non_table_entries_are_skipped(store):
    store.write_text('[templates]\nstray = "oops"\n')

    assert load_templates() == {}


def test_preselect_maps_labels_and_bare_dir_names():
    choice_to_path = {
        "alpha": Path("/code/alpha"),
        "nested/beta": Path("/code/nested/beta"),
    }
    template = Template(name="t", repos=["alpha", "beta"])

    # "beta" resolves via the bare directory name of "nested/beta".
    assert preselect_template_repos(template, choice_to_path) == ["alpha", "nested/beta"]


def test_preselect_skips_missing_repos_without_failing():
    choice_to_path = {"alpha": Path("/code/alpha")}
    template = Template(name="t", repos=["alpha", "gone"])

    assert preselect_template_repos(template, choice_to_path) == ["alpha"]


def test_preselect_dedupes_repeated_repos():
    choice_to_path = {"alpha": Path("/code/alpha")}
    template = Template(name="t", repos=["alpha", "alpha"])

    assert preselect_template_repos(template, choice_to_path) == ["alpha"]


def test_detail_prefers_description_then_repos():
    assert template_detail(Template(name="t", repos=["a"], description="desc")) == "desc"
    assert template_detail(Template(name="t", repos=["a", "b"])) == "a, b"


def test_detail_truncates_long_repo_lists():
    detail = template_detail(Template(name="t", repos=[f"repo-{i}" for i in range(20)]))
    assert len(detail) == 60
    assert detail.endswith("...")

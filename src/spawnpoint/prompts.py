"""Shared interactive prompts.

The repo picker is used by ``create``, ``add`` and ``template save``, so it lives
here rather than being duplicated per command.
"""

from typing import List, Sequence

from InquirerPy.base.control import Choice
from InquirerPy.prompts.fuzzy import FuzzyPrompt


class ClearOnToggleFuzzyPrompt(FuzzyPrompt):
    """FuzzyPrompt that clears search and shows selected repos on toggle."""

    def _handle_toggle_choice(self, _) -> None:
        super()._handle_toggle_choice(_)
        self._buffer.reset()
        # Reset filtered list to show all choices (not just previous search results)
        for choice in self.content_control.choices:
            choice["indices"] = []
        self.content_control._filtered_choices = self.content_control.choices

    def _generate_after_input(self):
        display = super()._generate_after_input()
        selected = self.selected_choices
        if selected:
            names = ", ".join(c["name"] for c in selected)
            display.append(("", "  "))
            display.append(("class:fuzzy_info", f"Selected: {names}"))
        return display


def select_repos(
    message: str,
    labels: Sequence[str],
    preselected: Sequence[str] = (),
) -> List[str]:
    """Fuzzy multi-select over repo labels; returns the selected labels.

    ``preselected`` labels start toggled on, so a template can seed the picker
    while still letting the user add or drop repos for this one workspace.
    """
    enabled = set(preselected)
    choices = [Choice(value=label, name=label, enabled=label in enabled) for label in labels]
    return ClearOnToggleFuzzyPrompt(
        message=message,
        choices=choices,
        multiselect=True,
    ).execute()

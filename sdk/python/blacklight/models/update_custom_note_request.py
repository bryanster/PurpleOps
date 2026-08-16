from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset
from typing import cast


T = TypeVar("T", bound="UpdateCustomNoteRequest")


@_attrs_define
class UpdateCustomNoteRequest:
    """Partial patch for a custom knowledge-base note."""

    title: str | Unset = UNSET
    body_markdown: str | Unset = UNSET
    tags: list[str] | Unset = UNSET
    technique_external_id: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        title = self.title

        body_markdown = self.body_markdown

        tags: list[str] | Unset = UNSET
        if not isinstance(self.tags, Unset):
            tags = self.tags

        technique_external_id = self.technique_external_id

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if title is not UNSET:
            field_dict["title"] = title
        if body_markdown is not UNSET:
            field_dict["bodyMarkdown"] = body_markdown
        if tags is not UNSET:
            field_dict["tags"] = tags
        if technique_external_id is not UNSET:
            field_dict["techniqueExternalId"] = technique_external_id

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        title = d.pop("title", UNSET)

        body_markdown = d.pop("bodyMarkdown", UNSET)

        tags = cast(list[str], d.pop("tags", UNSET))

        technique_external_id = d.pop("techniqueExternalId", UNSET)

        update_custom_note_request = cls(
            title=title,
            body_markdown=body_markdown,
            tags=tags,
            technique_external_id=technique_external_id,
        )

        update_custom_note_request.additional_properties = d
        return update_custom_note_request

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties

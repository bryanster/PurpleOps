from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from typing import cast
from uuid import UUID
import datetime


T = TypeVar("T", bound="ContentNote")


@_attrs_define
class ContentNote:
    """One freeform knowledge-base note under the custom source (or imported
    into it). Markdown body; optional technique link and tags.

    """

    id: UUID
    source_id: UUID
    version: str
    """ Version token. Custom is rolling-head and always `current`. """
    external_id: str
    """ Stable id within the custom source (defaults to the row id). """
    title: str
    body_markdown: str
    """ Markdown body. Size-capped by server config. """
    tags: list[str]
    technique_external_id: str
    """ Optional ATT&CK technique external id (`T1059`, `T1059.001`).
    Empty string when unset.
     """
    created_at: datetime.datetime
    updated_at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        source_id = str(self.source_id)

        version = self.version

        external_id = self.external_id

        title = self.title

        body_markdown = self.body_markdown

        tags = self.tags

        technique_external_id = self.technique_external_id

        created_at = self.created_at.isoformat()

        updated_at = self.updated_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "sourceId": source_id,
                "version": version,
                "externalId": external_id,
                "title": title,
                "bodyMarkdown": body_markdown,
                "tags": tags,
                "techniqueExternalId": technique_external_id,
                "createdAt": created_at,
                "updatedAt": updated_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        source_id = UUID(d.pop("sourceId"))

        version = d.pop("version")

        external_id = d.pop("externalId")

        title = d.pop("title")

        body_markdown = d.pop("bodyMarkdown")

        tags = cast(list[str], d.pop("tags"))

        technique_external_id = d.pop("techniqueExternalId")

        created_at = datetime.datetime.fromisoformat(d.pop("createdAt"))

        updated_at = datetime.datetime.fromisoformat(d.pop("updatedAt"))

        content_note = cls(
            id=id,
            source_id=source_id,
            version=version,
            external_id=external_id,
            title=title,
            body_markdown=body_markdown,
            tags=tags,
            technique_external_id=technique_external_id,
            created_at=created_at,
            updated_at=updated_at,
        )

        content_note.additional_properties = d
        return content_note

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

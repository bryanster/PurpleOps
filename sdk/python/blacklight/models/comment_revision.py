from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from typing import cast
from uuid import UUID
import datetime


T = TypeVar("T", bound="CommentRevision")


@_attrs_define
class CommentRevision:
    id: UUID
    comment_id: UUID
    body: str
    """ The body before this edit was applied. """
    edited_by: str
    """ User id of the editor. """
    edited_at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        comment_id = str(self.comment_id)

        body = self.body

        edited_by = self.edited_by

        edited_at = self.edited_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "commentId": comment_id,
                "body": body,
                "editedBy": edited_by,
                "editedAt": edited_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        comment_id = UUID(d.pop("commentId"))

        body = d.pop("body")

        edited_by = d.pop("editedBy")

        edited_at = datetime.datetime.fromisoformat(d.pop("editedAt"))

        comment_revision = cls(
            id=id,
            comment_id=comment_id,
            body=body,
            edited_by=edited_by,
            edited_at=edited_at,
        )

        comment_revision.additional_properties = d
        return comment_revision

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

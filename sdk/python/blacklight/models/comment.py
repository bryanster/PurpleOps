from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset
from typing import cast
from uuid import UUID
import datetime


T = TypeVar("T", bound="Comment")


@_attrs_define
class Comment:
    id: UUID
    execution_id: UUID
    author_id: str
    """ User id of the comment's author. """
    body: str
    """ Markdown or plain text. 16 KiB limit. """
    created_at: datetime.datetime
    edited_at: datetime.datetime | None | Unset = UNSET
    """ Null when never edited; set on first edit and updated thereafter. """
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        execution_id = str(self.execution_id)

        author_id = self.author_id

        body = self.body

        created_at = self.created_at.isoformat()

        edited_at: None | str | Unset
        if isinstance(self.edited_at, Unset):
            edited_at = UNSET
        elif isinstance(self.edited_at, datetime.datetime):
            edited_at = self.edited_at.isoformat()
        else:
            edited_at = self.edited_at

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "executionId": execution_id,
                "authorId": author_id,
                "body": body,
                "createdAt": created_at,
            }
        )
        if edited_at is not UNSET:
            field_dict["editedAt"] = edited_at

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        execution_id = UUID(d.pop("executionId"))

        author_id = d.pop("authorId")

        body = d.pop("body")

        created_at = datetime.datetime.fromisoformat(d.pop("createdAt"))

        def _parse_edited_at(data: object) -> datetime.datetime | None | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            try:
                if not isinstance(data, str):
                    raise TypeError()
                edited_at_type_0 = datetime.datetime.fromisoformat(data)

                return edited_at_type_0
            except (TypeError, ValueError, AttributeError, KeyError):
                pass
            return cast(datetime.datetime | None | Unset, data)

        edited_at = _parse_edited_at(d.pop("editedAt", UNSET))

        comment = cls(
            id=id,
            execution_id=execution_id,
            author_id=author_id,
            body=body,
            created_at=created_at,
            edited_at=edited_at,
        )

        comment.additional_properties = d
        return comment

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

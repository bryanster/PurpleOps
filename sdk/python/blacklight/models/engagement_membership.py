from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..models.engagement_role import EngagementRole
from typing import cast
import datetime


T = TypeVar("T", bound="EngagementMembership")


@_attrs_define
class EngagementMembership:
    """One person's place in one engagement."""

    engagement_id: str
    role: EngagementRole
    """ What somebody may do inside one engagement. Red and blue are separate so
    that blind mode and the split write endpoints have something to decide
    on.
     """
    added_at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        engagement_id = self.engagement_id

        role = self.role.value

        added_at = self.added_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "engagementId": engagement_id,
                "role": role,
                "addedAt": added_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        engagement_id = d.pop("engagementId")

        role = EngagementRole(d.pop("role"))

        added_at = datetime.datetime.fromisoformat(d.pop("addedAt"))

        engagement_membership = cls(
            engagement_id=engagement_id,
            role=role,
            added_at=added_at,
        )

        engagement_membership.additional_properties = d
        return engagement_membership

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

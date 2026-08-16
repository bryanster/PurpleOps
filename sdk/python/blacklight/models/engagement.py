from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..models.engagement_mode import EngagementMode
from ..models.engagement_status import EngagementStatus
from typing import cast
from uuid import UUID
import datetime


T = TypeVar("T", bound="Engagement")


@_attrs_define
class Engagement:
    id: UUID
    """ UUIDv7, sortable by creation time. """
    name: str
    client: str
    description: str
    status: EngagementStatus
    """ The lifecycle state of an assessment.
    `draft` → `active` → `closed` → `archived`, plus `draft` → `closed`.
     """
    starts_on: datetime.date
    ends_on: datetime.date
    attack_version: str
    """ Pinned ATT&CK version string, e.g. "15.1". """
    mode: EngagementMode
    """ `standard`: both sides see the workbook.
    `blind`: red/lead decide what blue sees via per-step reveal.
     """
    auto_reveal_on_start: bool
    """ When true, the first red transition reveals the step. """
    created_by: UUID
    created_at: datetime.datetime
    updated_at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        name = self.name

        client = self.client

        description = self.description

        status = self.status.value

        starts_on = self.starts_on.isoformat()

        ends_on = self.ends_on.isoformat()

        attack_version = self.attack_version

        mode = self.mode.value

        auto_reveal_on_start = self.auto_reveal_on_start

        created_by = str(self.created_by)

        created_at = self.created_at.isoformat()

        updated_at = self.updated_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "name": name,
                "client": client,
                "description": description,
                "status": status,
                "startsOn": starts_on,
                "endsOn": ends_on,
                "attackVersion": attack_version,
                "mode": mode,
                "autoRevealOnStart": auto_reveal_on_start,
                "createdBy": created_by,
                "createdAt": created_at,
                "updatedAt": updated_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        name = d.pop("name")

        client = d.pop("client")

        description = d.pop("description")

        status = EngagementStatus(d.pop("status"))

        starts_on = datetime.date.fromisoformat(d.pop("startsOn"))

        ends_on = datetime.date.fromisoformat(d.pop("endsOn"))

        attack_version = d.pop("attackVersion")

        mode = EngagementMode(d.pop("mode"))

        auto_reveal_on_start = d.pop("autoRevealOnStart")

        created_by = UUID(d.pop("createdBy"))

        created_at = datetime.datetime.fromisoformat(d.pop("createdAt"))

        updated_at = datetime.datetime.fromisoformat(d.pop("updatedAt"))

        engagement = cls(
            id=id,
            name=name,
            client=client,
            description=description,
            status=status,
            starts_on=starts_on,
            ends_on=ends_on,
            attack_version=attack_version,
            mode=mode,
            auto_reveal_on_start=auto_reveal_on_start,
            created_by=created_by,
            created_at=created_at,
            updated_at=updated_at,
        )

        engagement.additional_properties = d
        return engagement

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

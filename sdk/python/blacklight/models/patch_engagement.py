from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..models.engagement_mode import EngagementMode
from ..types import UNSET, Unset
from typing import cast
import datetime


T = TypeVar("T", bound="PatchEngagement")


@_attrs_define
class PatchEngagement:
    """Every field is optional; only the ones present are changed."""

    name: str | Unset = UNSET
    client: str | Unset = UNSET
    description: str | Unset = UNSET
    starts_on: datetime.date | Unset = UNSET
    ends_on: datetime.date | Unset = UNSET
    attack_version: str | Unset = UNSET
    mode: EngagementMode | Unset = UNSET
    """ `standard`: both sides see the workbook.
    `blind`: red/lead decide what blue sees via per-step reveal.
     """
    auto_reveal_on_start: bool | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        client = self.client

        description = self.description

        starts_on: str | Unset = UNSET
        if not isinstance(self.starts_on, Unset):
            starts_on = self.starts_on.isoformat()

        ends_on: str | Unset = UNSET
        if not isinstance(self.ends_on, Unset):
            ends_on = self.ends_on.isoformat()

        attack_version = self.attack_version

        mode: str | Unset = UNSET
        if not isinstance(self.mode, Unset):
            mode = self.mode.value

        auto_reveal_on_start = self.auto_reveal_on_start

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if name is not UNSET:
            field_dict["name"] = name
        if client is not UNSET:
            field_dict["client"] = client
        if description is not UNSET:
            field_dict["description"] = description
        if starts_on is not UNSET:
            field_dict["startsOn"] = starts_on
        if ends_on is not UNSET:
            field_dict["endsOn"] = ends_on
        if attack_version is not UNSET:
            field_dict["attackVersion"] = attack_version
        if mode is not UNSET:
            field_dict["mode"] = mode
        if auto_reveal_on_start is not UNSET:
            field_dict["autoRevealOnStart"] = auto_reveal_on_start

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        name = d.pop("name", UNSET)

        client = d.pop("client", UNSET)

        description = d.pop("description", UNSET)

        _starts_on = d.pop("startsOn", UNSET)
        starts_on: datetime.date | Unset
        if isinstance(_starts_on, Unset):
            starts_on = UNSET
        else:
            starts_on = datetime.date.fromisoformat(_starts_on)

        _ends_on = d.pop("endsOn", UNSET)
        ends_on: datetime.date | Unset
        if isinstance(_ends_on, Unset):
            ends_on = UNSET
        else:
            ends_on = datetime.date.fromisoformat(_ends_on)

        attack_version = d.pop("attackVersion", UNSET)

        _mode = d.pop("mode", UNSET)
        mode: EngagementMode | Unset
        if isinstance(_mode, Unset):
            mode = UNSET
        else:
            mode = EngagementMode(_mode)

        auto_reveal_on_start = d.pop("autoRevealOnStart", UNSET)

        patch_engagement = cls(
            name=name,
            client=client,
            description=description,
            starts_on=starts_on,
            ends_on=ends_on,
            attack_version=attack_version,
            mode=mode,
            auto_reveal_on_start=auto_reveal_on_start,
        )

        patch_engagement.additional_properties = d
        return patch_engagement

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

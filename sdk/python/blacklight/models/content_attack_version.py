from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..models.content_source_version_status import ContentSourceVersionStatus
from ..types import UNSET, Unset
from typing import cast
import datetime


T = TypeVar("T", bound="ContentAttackVersion")


@_attrs_define
class ContentAttackVersion:
    """One installed ATT&CK release as the pin surface exposes it. The
    `version` string is what engagements will store as `attack_version`
    (M3) — opaque equality to `content_source_version.version`.

    """

    version: str
    """ Release label (for example `15.1`). """
    status: ContentSourceVersionStatus
    """ State of one version snapshot under a source. """
    item_count: int
    source_enabled: bool
    """ Whether the ATT&CK source is currently enabled. A disabled source
    cannot accept new pins even when this version row is ready.
     """
    synced_at: datetime.datetime | Unset = UNSET
    """ When this version last finished successfully. Absent if never. """
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        version = self.version

        status = self.status.value

        item_count = self.item_count

        source_enabled = self.source_enabled

        synced_at: str | Unset = UNSET
        if not isinstance(self.synced_at, Unset):
            synced_at = self.synced_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "version": version,
                "status": status,
                "itemCount": item_count,
                "sourceEnabled": source_enabled,
            }
        )
        if synced_at is not UNSET:
            field_dict["syncedAt"] = synced_at

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        version = d.pop("version")

        status = ContentSourceVersionStatus(d.pop("status"))

        item_count = d.pop("itemCount")

        source_enabled = d.pop("sourceEnabled")

        _synced_at = d.pop("syncedAt", UNSET)
        synced_at: datetime.datetime | Unset
        if isinstance(_synced_at, Unset):
            synced_at = UNSET
        else:
            synced_at = datetime.datetime.fromisoformat(_synced_at)

        content_attack_version = cls(
            version=version,
            status=status,
            item_count=item_count,
            source_enabled=source_enabled,
            synced_at=synced_at,
        )

        content_attack_version.additional_properties = d
        return content_attack_version

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

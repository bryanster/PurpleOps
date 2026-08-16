from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..models.content_source_version_status import ContentSourceVersionStatus
from ..types import UNSET, Unset
from typing import cast
from uuid import UUID
import datetime


T = TypeVar("T", bound="ContentSourceVersion")


@_attrs_define
class ContentSourceVersion:
    """One version snapshot under a source."""

    id: UUID
    source_id: UUID
    version: str
    """ Release label for ATT&CK (e.g. `15.1`), or the rolling token
    `current` for Atomic / Sigma / CTID / custom.
     """
    status: ContentSourceVersionStatus
    """ State of one version snapshot under a source. """
    item_count: int
    error: str
    raw_sha_256: str
    """ Hex SHA-256 of the last successful raw snapshot. Empty if none. """
    raw_path: str
    """ Path of the raw snapshot relative to the content data root. """
    raw_bytes: int
    created_at: datetime.datetime
    updated_at: datetime.datetime
    synced_at: datetime.datetime | Unset = UNSET
    """ When this version last finished successfully. Absent if never. """
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        source_id = str(self.source_id)

        version = self.version

        status = self.status.value

        item_count = self.item_count

        error = self.error

        raw_sha_256 = self.raw_sha_256

        raw_path = self.raw_path

        raw_bytes = self.raw_bytes

        created_at = self.created_at.isoformat()

        updated_at = self.updated_at.isoformat()

        synced_at: str | Unset = UNSET
        if not isinstance(self.synced_at, Unset):
            synced_at = self.synced_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "sourceId": source_id,
                "version": version,
                "status": status,
                "itemCount": item_count,
                "error": error,
                "rawSha256": raw_sha_256,
                "rawPath": raw_path,
                "rawBytes": raw_bytes,
                "createdAt": created_at,
                "updatedAt": updated_at,
            }
        )
        if synced_at is not UNSET:
            field_dict["syncedAt"] = synced_at

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        source_id = UUID(d.pop("sourceId"))

        version = d.pop("version")

        status = ContentSourceVersionStatus(d.pop("status"))

        item_count = d.pop("itemCount")

        error = d.pop("error")

        raw_sha_256 = d.pop("rawSha256")

        raw_path = d.pop("rawPath")

        raw_bytes = d.pop("rawBytes")

        created_at = datetime.datetime.fromisoformat(d.pop("createdAt"))

        updated_at = datetime.datetime.fromisoformat(d.pop("updatedAt"))

        _synced_at = d.pop("syncedAt", UNSET)
        synced_at: datetime.datetime | Unset
        if isinstance(_synced_at, Unset):
            synced_at = UNSET
        else:
            synced_at = datetime.datetime.fromisoformat(_synced_at)

        content_source_version = cls(
            id=id,
            source_id=source_id,
            version=version,
            status=status,
            item_count=item_count,
            error=error,
            raw_sha_256=raw_sha_256,
            raw_path=raw_path,
            raw_bytes=raw_bytes,
            created_at=created_at,
            updated_at=updated_at,
            synced_at=synced_at,
        )

        content_source_version.additional_properties = d
        return content_source_version

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

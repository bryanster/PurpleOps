from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset


T = TypeVar("T", bound="StartContentSyncRequest")


@_attrs_define
class StartContentSyncRequest:
    """Optional pin for multi-version sources. Additional properties are
    rejected so a mistyped field cannot silently no-op.

    """

    version: str | Unset = UNSET
    """ ATT&CK release label (e.g. `15.1`). Omit for latest discoverable.
    Ignored by rolling sources.
     """

    def to_dict(self) -> dict[str, Any]:
        version = self.version

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if version is not UNSET:
            field_dict["version"] = version

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        version = d.pop("version", UNSET)

        start_content_sync_request = cls(
            version=version,
        )

        return start_content_sync_request

from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset
from typing import cast
import datetime


T = TypeVar("T", bound="ContentAttackRelease")


@_attrs_define
class ContentAttackRelease:
    """One ATT&CK release, as something to choose between."""

    version: str
    """ Upstream's own release label (for example `17.1`), and the string to
    send back as a sync pin. Not normalized: what upstream calls it is
    what a fetch will resolve.
     """
    installed: bool
    """ Whether this installation already holds a version row for this
    label, in any state. `status` says which.
     """
    latest: bool
    """ Upstream's newest release. Exactly one item carries `true` when the
    index was read, and none do when it was not.
     """
    released: datetime.datetime | Unset = UNSET
    """ When upstream published it. Absent when upstream does not say, and
    when the release is known only because it is installed here.
     """
    status: str | Unset = UNSET
    """ The installed version's state (`pending`, `syncing`, `ready`,
    `failed`). Absent when `installed` is false. A release that is
    installed but `failed` is still worth offering, which is why this is
    a status rather than a second boolean.
     """
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        version = self.version

        installed = self.installed

        latest = self.latest

        released: str | Unset = UNSET
        if not isinstance(self.released, Unset):
            released = self.released.isoformat()

        status = self.status

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "version": version,
                "installed": installed,
                "latest": latest,
            }
        )
        if released is not UNSET:
            field_dict["released"] = released
        if status is not UNSET:
            field_dict["status"] = status

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        version = d.pop("version")

        installed = d.pop("installed")

        latest = d.pop("latest")

        _released = d.pop("released", UNSET)
        released: datetime.datetime | Unset
        if isinstance(_released, Unset):
            released = UNSET
        else:
            released = datetime.datetime.fromisoformat(_released)

        status = d.pop("status", UNSET)

        content_attack_release = cls(
            version=version,
            installed=installed,
            latest=latest,
            released=released,
            status=status,
        )

        content_attack_release.additional_properties = d
        return content_attack_release

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

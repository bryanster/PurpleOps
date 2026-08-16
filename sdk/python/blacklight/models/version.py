from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset


T = TypeVar("T", bound="Version")


@_attrs_define
class Version:
    """Build identity of the running binary, stamped at link time. Every field is
    populated: an unstamped build reports a placeholder rather than an empty
    string.

    """

    version: str
    """ Release identifier from `git describe`, or `dev` for an unstamped build. """
    commit: str
    """ Git commit the binary was built from, or `unknown`. """
    build_date: str
    """ UTC build timestamp, RFC 3339 — or the literal `unknown` for a build
    without version stamping. Deliberately not `format: date-time`,
    because it is not always a timestamp and a client must not assume it
    parses.
     """
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        version = self.version

        commit = self.commit

        build_date = self.build_date

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "version": version,
                "commit": commit,
                "buildDate": build_date,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        version = d.pop("version")

        commit = d.pop("commit")

        build_date = d.pop("buildDate")

        version = cls(
            version=version,
            commit=commit,
            build_date=build_date,
        )

        version.additional_properties = d
        return version

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

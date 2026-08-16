from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset


T = TypeVar("T", bound="ClaimReportShare")


@_attrs_define
class ClaimReportShare:
    """Body of POST /report-views/{token}/claim."""

    password: str | Unset = UNSET
    """ Required if the share has a password gate. """

    def to_dict(self) -> dict[str, Any]:
        password = self.password

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if password is not UNSET:
            field_dict["password"] = password

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        password = d.pop("password", UNSET)

        claim_report_share = cls(
            password=password,
        )

        return claim_report_share

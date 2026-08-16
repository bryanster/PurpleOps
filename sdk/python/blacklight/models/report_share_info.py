from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset


T = TypeVar("T", bound="ReportShareInfo")


@_attrs_define
class ReportShareInfo:
    exists: bool
    """ Whether the share exists and is active. """
    password_required: bool | Unset = UNSET
    """ Whether the share requires a password. """
    already_claimed: bool | Unset = UNSET
    """ Whether the current user has already claimed this share. """
    label: str | Unset = UNSET
    """ The share's label, if set. """

    def to_dict(self) -> dict[str, Any]:
        exists = self.exists

        password_required = self.password_required

        already_claimed = self.already_claimed

        label = self.label

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "exists": exists,
            }
        )
        if password_required is not UNSET:
            field_dict["passwordRequired"] = password_required
        if already_claimed is not UNSET:
            field_dict["alreadyClaimed"] = already_claimed
        if label is not UNSET:
            field_dict["label"] = label

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        exists = d.pop("exists")

        password_required = d.pop("passwordRequired", UNSET)

        already_claimed = d.pop("alreadyClaimed", UNSET)

        label = d.pop("label", UNSET)

        report_share_info = cls(
            exists=exists,
            password_required=password_required,
            already_claimed=already_claimed,
            label=label,
        )

        return report_share_info

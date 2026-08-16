from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset


T = TypeVar("T", bound="MFAPolicy")


@_attrs_define
class MFAPolicy:
    """The platform-wide half of the multi-factor requirement (M1-008). The
    other half is the per-user `mfaEnforced` flag, and the effective
    requirement for one person is the **or** of whichever apply — so turning
    both of these off does not release somebody an administrator has
    individually enforced.

    Policy is evaluated before enrolment is looked at, which is the whole
    point: v1 asked "have they enrolled?" and so let anybody who skipped
    enrolment sign in with a password alone.

    """

    required_for_all: bool
    """ Every account that signs in with a local password must hold a second factor. """
    required_for_admins: bool
    """ Every account with the `admin` platform role must hold one. Implied
    by `requiredForAll`; the two are stored separately so that turning
    the wider one off does not silently release administrators too.
     """

    def to_dict(self) -> dict[str, Any]:
        required_for_all = self.required_for_all

        required_for_admins = self.required_for_admins

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "requiredForAll": required_for_all,
                "requiredForAdmins": required_for_admins,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        required_for_all = d.pop("requiredForAll")

        required_for_admins = d.pop("requiredForAdmins")

        mfa_policy = cls(
            required_for_all=required_for_all,
            required_for_admins=required_for_admins,
        )

        return mfa_policy

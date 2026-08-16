from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset


T = TypeVar("T", bound="ChangePasswordRequest")


@_attrs_define
class ChangePasswordRequest:
    """Body of `POST /auth/password`. The current password is required even
    though the caller is signed in: a session left open on a shared machine
    must not be enough to take the account over.

    """

    current_password: str
    new_password: str
    """ The replacement. The policy — at least 12 characters, at most 128,
    and not one of the passwords attackers try first — is applied by the
    server and reported as field errors on `newPassword`, so that there
    is one definition of an acceptable password rather than one here and
    one in the client.
     """

    def to_dict(self) -> dict[str, Any]:
        current_password = self.current_password

        new_password = self.new_password

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "currentPassword": current_password,
                "newPassword": new_password,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        current_password = d.pop("currentPassword")

        new_password = d.pop("newPassword")

        change_password_request = cls(
            current_password=current_password,
            new_password=new_password,
        )

        return change_password_request

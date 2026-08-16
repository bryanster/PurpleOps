from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset


T = TypeVar("T", bound="MFAState")


@_attrs_define
class MFAState:
    """Where this person and this session stand on multi-factor authentication.
    Whether an administrator *requires* it and whether this session has
    *satisfied* it are separate facts — conflating them is the hole M1-008
    closes.

    """

    enforced: bool
    """ An administrator requires a second factor of *this person
    specifically* — the per-user flag, set through the user
    administration API.

    It is one of the inputs to `required`, and not the whole answer:
    somebody can be required to hold a factor by the platform policy
    with this flag off. A client deciding whether to block should read
    `required`.
     """
    required: bool
    """ Whether this person must hold a second factor: `enforced`, or the
    platform policy (`GET /settings/mfa`) applying to them. This is the
    effective answer and the one an interface acts on (M1-008).

    `required` with `enrolled` false is the state that confines a
    session to enrolling: every other endpoint answers `403` with `code`
    `mfa_enrolment_required` until an enrolment is confirmed.

    An account that signs in only through an identity provider is
    exempt, and reports `false` — the provider owns its factors. See
    `docs/security.md`.
     """
    enrolled: bool
    """ This person has a confirmed authenticator (M1-006). A *started* but
    unconfirmed enrolment does not count — it gates nothing, so
    reporting it as enrolled would be a lie the interface acted on.

    Separate from `enforced` for the same reason `satisfied` is:
    "required to" and "has" are different facts, and treating one as
    evidence of the other is the hole M1-008 closes.
     """
    satisfied: bool
    """ A second factor was presented for *this* session. """
    recovery_codes_remaining: int
    """ How many unused recovery codes this person holds (M1-007). `0` for
    somebody who has not enrolled: codes are minted when an enrolment is
    confirmed and deleted when it is removed.

    A count and never the codes — those were shown once, and the server
    keeps only their hashes. Warn below three: the number matters most
    to the person who is one lost phone away from having no way in.
     """
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        enforced = self.enforced

        required = self.required

        enrolled = self.enrolled

        satisfied = self.satisfied

        recovery_codes_remaining = self.recovery_codes_remaining

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "enforced": enforced,
                "required": required,
                "enrolled": enrolled,
                "satisfied": satisfied,
                "recoveryCodesRemaining": recovery_codes_remaining,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        enforced = d.pop("enforced")

        required = d.pop("required")

        enrolled = d.pop("enrolled")

        satisfied = d.pop("satisfied")

        recovery_codes_remaining = d.pop("recoveryCodesRemaining")

        mfa_state = cls(
            enforced=enforced,
            required=required,
            enrolled=enrolled,
            satisfied=satisfied,
            recovery_codes_remaining=recovery_codes_remaining,
        )

        mfa_state.additional_properties = d
        return mfa_state

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
